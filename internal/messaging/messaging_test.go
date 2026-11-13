package messaging_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventAndDecode(t *testing.T) {
	env, err := messaging.NewEvent(messaging.EvtDiagnosisFinished, "saga-1", "os-1", "corr-1",
		messaging.DiagnosisFinishedPayload{Diagnosis: "Troca de óleo"})
	require.NoError(t, err)
	assert.NotEmpty(t, env.MessageID)
	assert.NotEmpty(t, env.OccurredAt)
	assert.Equal(t, "saga-1", env.SagaID)

	var p messaging.DiagnosisFinishedPayload
	require.NoError(t, env.DecodePayload(&p))
	assert.Equal(t, "Troca de óleo", p.Diagnosis)
}

func TestNewEventWithoutPayload(t *testing.T) {
	env, err := messaging.NewEvent(messaging.EvtPartsReleased, "s", "o", "c", nil)
	require.NoError(t, err)
	assert.Empty(t, env.Payload)
	assert.ErrorIs(t, env.DecodePayload(&struct{}{}), messaging.ErrEmptyPayload)
}

func TestNewEventInvalidPayload(t *testing.T) {
	_, err := messaging.NewEvent("x", "s", "o", "c", make(chan int))
	assert.Error(t, err)
}

func TestReplyToKeepsSagaContext(t *testing.T) {
	cmd := messaging.Envelope{MessageID: "m1", Type: messaging.CmdReserveParts, SagaID: "saga-1", OrderID: "os-1", CorrelationID: "corr-1"}
	evt, err := messaging.ReplyTo(cmd, messaging.EvtPartsReserved, nil)
	require.NoError(t, err)
	assert.Equal(t, messaging.EvtPartsReserved, evt.Type)
	assert.Equal(t, "saga-1", evt.SagaID)
	assert.Equal(t, "os-1", evt.OrderID)
	assert.Equal(t, "corr-1", evt.CorrelationID)
	assert.NotEqual(t, "m1", evt.MessageID)
}

func TestReservePartsPayloadJSON(t *testing.T) {
	raw := `{"items":[{"part_id":"p1","quantity":2}]}`
	var p messaging.ReservePartsPayload
	require.NoError(t, json.Unmarshal([]byte(raw), &p))
	assert.Equal(t, []domain.ReservationItem{{PartID: "p1", Quantity: 2}}, p.Items)
}

func TestPublisher_SendsEnvelope(t *testing.T) {
	fake := &mocks.SQSFake{}
	pub := messaging.NewSQSPublisher(fake, "http://queue/os-events")
	env, _ := messaging.NewEvent(messaging.EvtPartsReserved, "s", "o", "corr-1", nil)

	require.NoError(t, pub.Publish(context.Background(), env))
	require.Len(t, fake.Sent, 1)
	assert.Equal(t, "http://queue/os-events", aws.ToString(fake.Sent[0].QueueUrl))
	assert.Equal(t, "corr-1", aws.ToString(fake.Sent[0].MessageAttributes["correlation_id"].StringValue))

	var sent messaging.Envelope
	require.NoError(t, json.Unmarshal([]byte(aws.ToString(fake.Sent[0].MessageBody)), &sent))
	assert.Equal(t, env.MessageID, sent.MessageID)
}

func TestPublisher_RetriesThenSucceeds(t *testing.T) {
	fake := &mocks.SQSFake{SendErrs: []error{errors.New("falha momentânea")}}
	pub := messaging.NewSQSPublisher(fake, "q")
	env, _ := messaging.NewEvent("x", "s", "o", "", nil)

	require.NoError(t, pub.Publish(context.Background(), env))
	assert.Len(t, fake.Sent, 1)
	_, hasCorrelation := fake.Sent[0].MessageAttributes["correlation_id"]
	assert.False(t, hasCorrelation)
}

func TestPublisher_GivesUpAfterAttempts(t *testing.T) {
	boom := errors.New("fora do ar")
	fake := &mocks.SQSFake{SendErrs: []error{boom, boom, boom}}
	pub := messaging.NewSQSPublisher(fake, "q")
	env, _ := messaging.NewEvent("x", "s", "o", "c", nil)

	err := pub.Publish(context.Background(), env)
	assert.ErrorIs(t, err, boom)
	assert.Empty(t, fake.Sent)
}

func TestQueueURL(t *testing.T) {
	fake := &mocks.SQSFake{QueueURLs: map[string]string{"os-events": "http://queue/os-events"}}
	url, err := messaging.QueueURL(context.Background(), fake, "os-events")
	require.NoError(t, err)
	assert.Equal(t, "http://queue/os-events", url)

	_, err = messaging.QueueURL(context.Background(), fake, "nao-existe")
	assert.Error(t, err)
}
