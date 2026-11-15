// Package messaging define o formato das mensagens trocadas com o
// orquestrador da Saga (ADR-006) e a publicação no SQS.
package messaging

import (
	"encoding/json"
	"errors"
	"time"

	"execution-autoshop/internal/domain"

	"github.com/google/uuid"
)

// Comandos recebidos do orquestrador
const (
	CmdStartDiagnosis = "StartDiagnosis"
	CmdReserveParts   = "ReserveParts"
	CmdReleaseParts   = "ReleaseParts"
	CmdStartExecution = "StartExecution"
)

// Eventos enviados ao orquestrador
const (
	EvtDiagnosisFinished      = "DiagnosisFinished"
	EvtPartsReserved          = "PartsReserved"
	EvtPartsReservationFailed = "PartsReservationFailed"
	EvtPartsReleased          = "PartsReleased"
	EvtExecutionQueued        = "ExecutionQueued"
	EvtExecutionStarted       = "ExecutionStarted"
	EvtExecutionFinished      = "ExecutionFinished"
)

var ErrEmptyPayload = errors.New("mensagem sem payload")

// Envelope é o formato comum de todas as mensagens da Saga.
type Envelope struct {
	MessageID     string          `json:"message_id"`
	Type          string          `json:"type"`
	SagaID        string          `json:"saga_id"`
	OrderID       string          `json:"order_id"`
	CorrelationID string          `json:"correlation_id"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

type ReservePartsPayload struct {
	Items []domain.ReservationItem `json:"items"`
}

type PartsReservationFailedPayload struct {
	Reason string `json:"reason"`
	PartID string `json:"part_id,omitempty"`
}

type DiagnosisFinishedPayload struct {
	Diagnosis string `json:"diagnosis"`
}

// NewEvent monta um evento com ID e data próprios, mantendo a Saga, a OS e
// o correlation_id de origem.
func NewEvent(eventType, sagaID, orderID, correlationID string, payload any) (Envelope, error) {
	env := Envelope{
		MessageID:     uuid.NewString(),
		Type:          eventType,
		SagaID:        sagaID,
		OrderID:       orderID,
		CorrelationID: correlationID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, err
		}
		env.Payload = raw
	}
	return env, nil
}

// ReplyTo monta o evento de resposta a um comando recebido.
func ReplyTo(cmd Envelope, eventType string, payload any) (Envelope, error) {
	return NewEvent(eventType, cmd.SagaID, cmd.OrderID, cmd.CorrelationID, payload)
}

func (e Envelope) DecodePayload(v any) error {
	if len(e.Payload) == 0 {
		return ErrEmptyPayload
	}
	return json.Unmarshal(e.Payload, v)
}
