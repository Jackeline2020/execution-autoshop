# Execution Service — AutoShop

Microsserviço de **execução e produção** da oficina: diagnóstico, peças e
estoque, reserva de peças e fila de execução dos reparos.

Faz parte da arquitetura de microsserviços do AutoShop. A visão geral, o
fluxo da Saga e as decisões de arquitetura estão no repositório
[`app-autoshop`](https://github.com/Jackeline2020/app-autoshop/tree/develop/docs):
[arquitetura](https://github.com/Jackeline2020/app-autoshop/blob/develop/docs/architecture.md),
[fluxo da OS](https://github.com/Jackeline2020/app-autoshop/blob/develop/docs/saga-flow.md) e
[banco por serviço](https://github.com/Jackeline2020/app-autoshop/blob/develop/docs/adr/ADR-008-banco-por-servico.md).

## Responsabilidades

- Cadastro de peças e controle de estoque
- Reserva de peças para uma OS, liberação (compensação da Saga) e consumo ao fim do reparo
- Diagnóstico da OS pelo mecânico
- Fila de trabalho da oficina: início, acompanhamento e finalização dos reparos

## Tecnologias

| Tecnologia | Uso |
|---|---|
| Go + Gin | API HTTP |
| DynamoDB | Banco do serviço (tabelas `parts`, `reservations`, `executions`, `processed_messages`) |
| Amazon SQS | Comandos da Saga (`execution-commands`) e eventos para o orquestrador (`os-events`) |
| New Relic | APM, logs JSON com `correlation_id` |
| LocalStack | DynamoDB e SQS em desenvolvimento e nos testes do pipeline |

## API

Todas as rotas, exceto `/health` e `/swagger`, exigem `Authorization: Bearer <token>`.
Documentação interativa em `/swagger/index.html`.

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/parts` | Cadastra peça |
| `GET` | `/parts` | Lista peças |
| `GET` | `/parts/low-stock` | Peças com estoque no mínimo ou abaixo |
| `GET` | `/parts/{id}` | Consulta peça |
| `PUT` | `/parts/{id}` | Atualiza peça |
| `PATCH` | `/parts/{id}/stock` | Entrada ou saída manual de estoque |
| `DELETE` | `/parts/{id}` | Remove peça |
| `GET` | `/executions/queue` | Fila da oficina (aguardando diagnóstico ou reparo), mais antigas primeiro |
| `GET` | `/executions/{order_id}` | Andamento da OS |
| `POST` | `/executions/{order_id}/diagnosis` | Registra o diagnóstico |
| `POST` | `/executions/{order_id}/start` | Inicia o reparo |
| `POST` | `/executions/{order_id}/finish` | Finaliza o reparo e consome as peças reservadas |

## Integração com a Saga

| Comando recebido | Ação | Evento enviado |
|---|---|---|
| `StartDiagnosis` | Coloca a OS na fila de diagnóstico | — (aguarda o mecânico) |
| — | Mecânico registra o diagnóstico | `DiagnosisFinished` |
| `ReserveParts` | Reserva as peças (tudo ou nada) | `PartsReserved` ou `PartsReservationFailed` |
| `ReleaseParts` | Devolve o estoque (compensação) | `PartsReleased` |
| `StartExecution` | Coloca a OS na fila de reparo | `ExecutionQueued` |
| — | Mecânico inicia o reparo | `ExecutionStarted` |
| — | Mecânico finaliza o reparo | `ExecutionFinished` |

- **Reserva atômica:** todas as peças da OS são reservadas em uma única
  transação do DynamoDB, com a condição `estoque >= quantidade`. Duas OS
  nunca reservam a mesma unidade.
- **Idempotência:** mensagens repetidas são ignoradas (`processed_messages`),
  e reservar ou liberar duas vezes não altera o estoque em dobro.
- **Falhas:** erros técnicos deixam a mensagem na fila para nova tentativa;
  após 5 tentativas ela vai para a DLQ.

## Como rodar localmente

Pré-requisitos: Docker.

```bash
docker compose up --build
curl http://localhost:8082/health
```

O `docker compose` sobe o LocalStack com as tabelas e filas do serviço
(`localstack/init-aws.sh`) e o serviço na porta `8082`. A pasta
`localstack/samples/` tem mensagens de exemplo para testar a integração
com a Saga:

```bash
docker compose exec localstack awslocal dynamodb put-item --table-name parts --item file:///samples/part.json
docker compose exec localstack awslocal sqs send-message \
  --queue-url http://localhost:4566/000000000000/execution-commands \
  --message-body file:///samples/reserve-parts.json
docker compose exec localstack awslocal sqs receive-message \
  --queue-url http://localhost:4566/000000000000/os-events
```

## Testes

Os testes de integração rodam contra o LocalStack e são ignorados quando
`AWS_ENDPOINT_URL` não está definida.

```bash
# unitários
go test ./... -cover

# unitários + integração
docker compose up -d localstack
AWS_ENDPOINT_URL=http://localhost:4566 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
  go test ./... -coverprofile=coverage.out
./scripts/coverage.sh
```

## Cobertura e qualidade

O pipeline exige **cobertura mínima de 80%** (`scripts/coverage.sh`) e
envia a análise para o SonarCloud.

<!-- Evidências: adicionar print do SonarCloud e link do projeto -->

## CI/CD

Pipeline em `.github/workflows/ci-cd.yml`, executado em todo pull request
e em push para `develop` e `main`:

1. Build e `go vet`
2. Testes unitários e de integração (LocalStack no próprio pipeline)
3. Verificação de cobertura mínima
4. SonarCloud
5. Build e publicação da imagem Docker no GitHub Container Registry

As branches `main` e `develop` são protegidas: alterações só entram por
pull request, com as checagens acima aprovadas.
