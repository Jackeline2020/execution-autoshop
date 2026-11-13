#!/bin/bash
# Cria no LocalStack as tabelas e filas usadas pelo Execution Service.
# Em produção, os mesmos recursos são criados por Terraform.
set -e

for table in parts:part_id reservations:order_id executions:order_id processed_messages:message_id; do
  name=${table%%:*}; key=${table##*:}
  awslocal dynamodb create-table \
    --table-name "$name" \
    --attribute-definitions AttributeName="$key",AttributeType=S \
    --key-schema AttributeName="$key",KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null
  echo "tabela $name criada"
done

create_queue_with_dlq() {
  local queue=$1
  local dlq_url dlq_arn
  dlq_url=$(awslocal sqs create-queue --queue-name "$queue-dlq" --query QueueUrl --output text)
  dlq_arn=$(awslocal sqs get-queue-attributes --queue-url "$dlq_url" --attribute-names QueueArn --query Attributes.QueueArn --output text)
  awslocal sqs create-queue --queue-name "$queue" --attributes "{
    \"VisibilityTimeout\": \"30\",
    \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"$dlq_arn\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"
  }" >/dev/null
  echo "fila $queue criada (DLQ: $queue-dlq)"
}

# Fila de entrada do serviço
create_queue_with_dlq execution-commands
# Fila de saída (eventos para o orquestrador). Criada aqui para permitir
# rodar o serviço isolado; no ambiente completo ela pertence ao OS Service.
create_queue_with_dlq os-events
