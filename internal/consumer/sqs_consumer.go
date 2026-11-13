package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/repository"
	"execution-autoshop/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Handler processa um comando já decodificado.
type Handler interface {
	Handle(ctx context.Context, cmd messaging.Envelope) error
}

// SQSConsumer lê a fila de comandos continuamente. Uma mensagem só é
// apagada depois de processada; se falhar, volta para a fila após o tempo
// de visibilidade e, depois de 5 tentativas, vai para a DLQ.
type SQSConsumer struct {
	client      messaging.SQSAPI
	queueURL    string
	handler     Handler
	processed   repository.ProcessedMessageRepository
	waitSeconds int32
	retryDelay  time.Duration
}

func NewSQSConsumer(client messaging.SQSAPI, queueURL string, h Handler, p repository.ProcessedMessageRepository) *SQSConsumer {
	return &SQSConsumer{client: client, queueURL: queueURL, handler: h, processed: p, waitSeconds: 20, retryDelay: 2 * time.Second}
}

// Run lê a fila até o contexto ser cancelado (encerramento do serviço).
func (c *SQSConsumer) Run(ctx context.Context) {
	logger.Info("consumidor iniciado", "queue", c.queueURL)
	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     c.waitSeconds,
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			logger.Error("erro ao ler a fila", "queue", c.queueURL, "error", err.Error())
			select {
			case <-ctx.Done():
			case <-time.After(c.retryDelay):
			}
			continue
		}
		for _, m := range out.Messages {
			c.Process(ctx, m)
		}
	}
	logger.Info("consumidor encerrado", "queue", c.queueURL)
}

// Process trata uma mensagem: ignora repetidas, executa o comando e apaga a
// mensagem quando não há mais nada a fazer com ela.
func (c *SQSConsumer) Process(ctx context.Context, m types.Message) {
	var cmd messaging.Envelope
	if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &cmd); err != nil || cmd.MessageID == "" {
		logger.Error("mensagem inválida descartada", "body", aws.ToString(m.Body))
		c.delete(ctx, m)
		return
	}
	attrs := []any{"type", cmd.Type, "message_id", cmd.MessageID, "order_id", cmd.OrderID, "correlation_id", cmd.CorrelationID}

	done, err := c.processed.Exists(cmd.MessageID)
	if err != nil {
		logger.Warn("erro ao verificar mensagem; será tentada de novo", append(attrs, "error", err.Error())...)
		return
	}
	if done {
		logger.Info("mensagem repetida ignorada", attrs...)
		c.delete(ctx, m)
		return
	}

	err = c.handler.Handle(ctx, cmd)
	switch {
	case errors.Is(err, ErrPermanent):
		logger.Error("comando descartado", append(attrs, "error", err.Error())...)
	case err != nil:
		logger.Warn("falha ao processar comando; será tentado de novo", append(attrs, "error", err.Error())...)
		return
	default:
		logger.Info("comando processado", attrs...)
	}

	if err := c.processed.MarkProcessed(cmd.MessageID); err != nil {
		logger.Warn("erro ao registrar mensagem processada", append(attrs, "error", err.Error())...)
	}
	c.delete(ctx, m)
}

func (c *SQSConsumer) delete(ctx context.Context, m types.Message) {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	})
	if err != nil {
		logger.Warn("erro ao apagar mensagem da fila", "error", err.Error())
	}
}
