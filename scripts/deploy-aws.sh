#!/bin/sh
# Deploy do serviço no cluster EKS, chamado pelo job deploy-aws do pipeline.
#
# Variáveis esperadas:
#   SERVICE         nome do Deployment (e da imagem nos manifestos)
#   IMAGE           imagem publicada nesta execução (ghcr.io/...:sha)
#   RUN_MIGRATIONS  "true" para aplicar as migrations do PostgreSQL
#   CONFIG_NAME     ConfigMap do serviço (padrão: <SERVICE>-config)
#   SECRET_NAME     Secret do serviço (padrão: <SERVICE>-secrets)
#   e os secrets que os manifestos usarem: IRSA_ROLE_ARN, DB_HOST,
#   DB_PASSWORD, JWT_SECRET, NEW_RELIC_LICENSE_KEY
set -eu

OVERLAY=k8s/overlays/aws
NAMESPACE=autoshop
CONFIG_NAME=${CONFIG_NAME:-${SERVICE}-config}
SECRET_NAME=${SECRET_NAME:-${SERVICE}-secrets}

# 1) Troca os *_PLACEHOLDER dos manifestos pelos secrets do repositório
for VAR in IRSA_ROLE_ARN DB_HOST DB_PASSWORD JWT_SECRET NEW_RELIC_LICENSE_KEY; do
  if grep -rq "${VAR}_PLACEHOLDER" "$OVERLAY"; then
    VALUE=$(printenv "$VAR" || true)
    if [ -z "$VALUE" ]; then
      echo "Secret $VAR não configurado no repositório" >&2
      exit 1
    fi
    sed -i "s#${VAR}_PLACEHOLDER#${VALUE}#g" "$OVERLAY"/*.yaml
  fi
done

# 2) Usa a imagem publicada nesta execução
cat >> "$OVERLAY/kustomization.yaml" <<IMAGES

images:
  - name: ${SERVICE}
    newName: ${IMAGE%:*}
    newTag: "${IMAGE##*:}"
IMAGES

# 3) Aplica os manifestos (Deployment, Service, ConfigMap, Secret, ServiceAccount)
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k "$OVERLAY"

# 4) Migrations do PostgreSQL, num Job com as credenciais do próprio serviço
if [ "${RUN_MIGRATIONS:-false}" = "true" ]; then
  kubectl create configmap "${SERVICE}-migrations" -n "$NAMESPACE" \
    --from-file=migrations/ --dry-run=client -o yaml | kubectl apply -f -
  kubectl delete job "${SERVICE}-migrate" -n "$NAMESPACE" --ignore-not-found

  cat <<JOB | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${SERVICE}-migrate
  namespace: ${NAMESPACE}
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: migrate/migrate:v4.17.1
          args:
            - "-path=/migrations"
            - "-database=postgres://\$(DB_USER):\$(DB_PASSWORD)@\$(DB_HOST):\$(DB_PORT)/\$(DB_NAME)?sslmode=require"
            - "up"
          envFrom:
            - configMapRef:
                name: ${CONFIG_NAME}
            - secretRef:
                name: ${SECRET_NAME}
          volumeMounts:
            - name: migrations
              mountPath: /migrations
      volumes:
        - name: migrations
          configMap:
            name: ${SERVICE}-migrations
JOB

  kubectl wait --for=condition=complete "job/${SERVICE}-migrate" -n "$NAMESPACE" --timeout=180s
fi

# 5) Aguarda a nova versão subir
kubectl rollout status "deployment/${SERVICE}" -n "$NAMESPACE" --timeout=300s
kubectl get pods,svc -n "$NAMESPACE" -l "app=${SERVICE}"
