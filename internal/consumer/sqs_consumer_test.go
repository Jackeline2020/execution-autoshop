package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"execution-autoshop/internal/consumer"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type handlerFunc func(ctx context.Context, cmd messaging.Envelope) error

func (f handlerFunc) Handle(ctx context.Context, cmd messaging.Envelope) error { return f(ctx, cmd) }

func sqsMessage(t *testing.T, id string) types.Message {
	t.Helper()
	body, _ := json.Marshal(messaging.Envelope{MessageID: id, Type: messaging.CmdReleaseParts, OrderID: "os-1"})
	return types.Message{Body: aws.String(string(body)), ReceiptHandle: aws.String("rh-" + id)}
}

func TestProcess_SuccessMarksAndDeletes(t *testing.T) {
	fake := &mocks.SQSFake{}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", "m1").Return(false, nil)
	processed.On("MarkProcessed", "m1").Return(nil)
	calls := 0
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { calls++; return nil }), processed)

	c.Process(context.Background(), sqsMessage(t, "m1"))
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"rh-m1"}, fake.Deleted)
	processed.AssertExpectations(t)
}

func TestProcess_DuplicateIsSkipped(t *testing.T) {
	fake := &mocks.SQSFake{}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", "m1").Return(true, nil)
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error {
		t.Fatal("não deveria processar mensagem repetida")
		return nil
	}), processed)

	c.Process(context.Background(), sqsMessage(t, "m1"))
	assert.Equal(t, 1, fake.DeletedCount())
}

func TestProcess_RetryableErrorKeepsMessage(t *testing.T) {
	fake := &mocks.SQSFake{}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", "m1").Return(false, nil)
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { return errors.New("dynamo fora") }), processed)

	c.Process(context.Background(), sqsMessage(t, "m1"))
	assert.Zero(t, fake.DeletedCount())
	processed.AssertNotCalled(t, "MarkProcessed", mock.Anything)
}

func TestProcess_PermanentErrorDeletes(t *testing.T) {
	fake := &mocks.SQSFake{}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", "m1").Return(false, nil)
	processed.On("MarkProcessed", "m1").Return(errors.New("falhou ao marcar"))
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { return consumer.ErrPermanent }), processed)

	c.Process(context.Background(), sqsMessage(t, "m1"))
	assert.Equal(t, 1, fake.DeletedCount())
}

func TestProcess_InvalidMessageIsDiscarded(t *testing.T) {
	fake := &mocks.SQSFake{}
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { return nil }), new(mocks.ProcessedMessageRepositoryMock))

	c.Process(context.Background(), types.Message{Body: aws.String("não é json"), ReceiptHandle: aws.String("rh-x")})
	c.Process(context.Background(), types.Message{Body: aws.String(`{"type":"X"}`), ReceiptHandle: aws.String("rh-y")})
	assert.Equal(t, []string{"rh-x", "rh-y"}, fake.Deleted)
}

func TestProcess_ExistsErrorKeepsMessage(t *testing.T) {
	fake := &mocks.SQSFake{}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", "m1").Return(false, errors.New("dynamo fora"))
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { return nil }), processed)

	c.Process(context.Background(), sqsMessage(t, "m1"))
	assert.Zero(t, fake.DeletedCount())
}

func TestRun_ProcessesUntilCancelled(t *testing.T) {
	fake := &mocks.SQSFake{Receives: []mocks.ReceiveResult{
		{Err: errors.New("falha momentânea")},
		{Messages: []types.Message{sqsMessage(t, "m1"), sqsMessage(t, "m2")}},
	}}
	processed := new(mocks.ProcessedMessageRepositoryMock)
	processed.On("Exists", mock.Anything).Return(false, nil)
	processed.On("MarkProcessed", mock.Anything).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	c := consumer.NewSQSConsumer(fake, "q", handlerFunc(func(context.Context, messaging.Envelope) error { return nil }), processed)

	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	assert.Eventually(t, func() bool { return fake.DeletedCount() == 2 }, 5*time.Second, 20*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("consumidor não encerrou após o cancelamento")
	}
}
