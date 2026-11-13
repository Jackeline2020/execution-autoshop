# Execution Service — AutoShop

Microsserviço de **execução e produção** da oficina: diagnóstico, peças e
estoque, reserva de peças e fila de execução dos reparos. Faz parte da
arquitetura de microsserviços do AutoShop, descrita no repositório
[`app-autoshop`](https://github.com/Jackeline2020/app-autoshop/blob/develop/docs/architecture.md).

> Em construção (Fase 4).

## Como rodar localmente

Pré-requisitos: Docker e Go 1.26.

```bash
docker compose up --build
curl http://localhost:8082/health
```

O `docker compose` sobe o LocalStack com as tabelas DynamoDB e as filas
SQS do serviço (`localstack/init-aws.sh`) e o próprio serviço na porta
`8082`.

## Testes

```bash
go test ./... -cover
```
