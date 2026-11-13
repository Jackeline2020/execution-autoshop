package mocks

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSFake simula o SQS em memória para os testes.
type SQSFake struct {
	mu        sync.Mutex
	Sent      []*sqs.SendMessageInput
	Deleted   []string
	SendErrs  []error // erros devolvidos nas próximas chamadas de SendMessage
	Receives  []ReceiveResult
	QueueURLs map[string]string
}

type ReceiveResult struct {
	Messages []types.Message
	Err      error
}

func (f *SQSFake) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.SendErrs) > 0 {
		err := f.SendErrs[0]
		f.SendErrs = f.SendErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	f.Sent = append(f.Sent, in)
	return &sqs.SendMessageOutput{MessageId: aws.String("id")}, nil
}

// ReceiveMessage devolve os resultados programados em Receives; quando
// acabam, espera o contexto ser cancelado.
func (f *SQSFake) ReceiveMessage(ctx context.Context, _ *sqs.ReceiveMessageInput, _ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	if len(f.Receives) > 0 {
		r := f.Receives[0]
		f.Receives = f.Receives[1:]
		f.mu.Unlock()
		return &sqs.ReceiveMessageOutput{Messages: r.Messages}, r.Err
	}
	f.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (f *SQSFake) DeleteMessage(_ context.Context, in *sqs.DeleteMessageInput, _ ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Deleted = append(f.Deleted, aws.ToString(in.ReceiptHandle))
	return &sqs.DeleteMessageOutput{}, nil
}

func (f *SQSFake) GetQueueUrl(_ context.Context, in *sqs.GetQueueUrlInput, _ ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error) {
	url, ok := f.QueueURLs[aws.ToString(in.QueueName)]
	if !ok {
		return nil, &types.QueueDoesNotExist{}
	}
	return &sqs.GetQueueUrlOutput{QueueUrl: aws.String(url)}, nil
}

func (f *SQSFake) DeletedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Deleted)
}
