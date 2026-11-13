package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSAPI reúne as operações do SQS usadas pelo serviço (permite testar
// sem a AWS).
type SQSAPI interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, opts ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, opts ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, opts ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	GetQueueUrl(ctx context.Context, in *sqs.GetQueueUrlInput, opts ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
}

// SQSPublisher envia eventos para uma fila, com algumas tentativas em caso
// de falha momentânea.
type SQSPublisher struct {
	client   SQSAPI
	queueURL string
	attempts int
	backoff  time.Duration
}

func NewSQSPublisher(client SQSAPI, queueURL string) *SQSPublisher {
	return &SQSPublisher{client: client, queueURL: queueURL, attempts: 3, backoff: 200 * time.Millisecond}
}

func (p *SQSPublisher) Publish(ctx context.Context, env Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}

	attrs := map[string]types.MessageAttributeValue{
		"type": {DataType: aws.String("String"), StringValue: aws.String(env.Type)},
	}
	if env.CorrelationID != "" {
		attrs["correlation_id"] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(env.CorrelationID)}
	}

	for attempt := 1; ; attempt++ {
		_, err = p.client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:          aws.String(p.queueURL),
			MessageBody:       aws.String(string(body)),
			MessageAttributes: attrs,
		})
		if err == nil || attempt >= p.attempts {
			break
		}
		time.Sleep(p.backoff * time.Duration(attempt))
	}
	if err != nil {
		return fmt.Errorf("erro ao publicar %s: %w", env.Type, err)
	}
	return nil
}

// QueueURL descobre a URL de uma fila pelo nome.
func QueueURL(ctx context.Context, client SQSAPI, name string) (string, error) {
	out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("fila %s não encontrada: %w", name, err)
	}
	return aws.ToString(out.QueueUrl), nil
}
