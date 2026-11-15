// Package consumer processa os comandos que o orquestrador da Saga envia
// para a fila execution-commands.
package consumer

import (
	"context"
	"errors"
	"fmt"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/usecase"
)

// ErrPermanent marca falhas que não se resolvem com nova tentativa (comando
// desconhecido, OS em estado incompatível). A mensagem é descartada e o
// prazo do passo no orquestrador trata o caso.
var ErrPermanent = errors.New("falha permanente")

type CommandHandler struct {
	executions   *usecase.ExecutionUseCase
	reservations *usecase.ReservationUseCase
	publisher    usecase.EventPublisher
}

func NewCommandHandler(e *usecase.ExecutionUseCase, r *usecase.ReservationUseCase, p usecase.EventPublisher) *CommandHandler {
	return &CommandHandler{executions: e, reservations: r, publisher: p}
}

// Handle executa o comando. Erros não marcados como ErrPermanent fazem a
// mensagem voltar para a fila e ser tentada de novo.
func (h *CommandHandler) Handle(ctx context.Context, cmd messaging.Envelope) error {
	switch cmd.Type {
	case messaging.CmdStartDiagnosis:
		_, err := h.executions.StartDiagnosis(cmd.OrderID, cmd.SagaID, cmd.CorrelationID)
		return permanentIf(err, domain.ErrOrderRequired)

	case messaging.CmdReserveParts:
		return h.reserveParts(ctx, cmd)

	case messaging.CmdReleaseParts:
		if err := h.reservations.Release(cmd.OrderID); err != nil {
			return err
		}
		return h.reply(ctx, cmd, messaging.EvtPartsReleased, nil)

	case messaging.CmdStartExecution:
		_, err := h.executions.Enqueue(cmd.OrderID)
		var transErr *domain.InvalidTransitionError
		if errors.Is(err, domain.ErrExecutionNotFound) || errors.As(err, &transErr) {
			return fmt.Errorf("%w: %v", ErrPermanent, err)
		}
		if err != nil {
			return err
		}
		return h.reply(ctx, cmd, messaging.EvtExecutionQueued, nil)
	}

	return fmt.Errorf("%w: comando desconhecido %q", ErrPermanent, cmd.Type)
}

// reserveParts responde PartsReserved ou, se faltar estoque ou o comando
// for inválido, PartsReservationFailed. Falhas técnicas são retentadas.
func (h *CommandHandler) reserveParts(ctx context.Context, cmd messaging.Envelope) error {
	var payload messaging.ReservePartsPayload
	if err := cmd.DecodePayload(&payload); err != nil {
		return h.reply(ctx, cmd, messaging.EvtPartsReservationFailed,
			messaging.PartsReservationFailedPayload{Reason: "comando inválido: " + err.Error()})
	}

	_, err := h.reservations.Reserve(cmd.OrderID, payload.Items)
	var stockErr *domain.InsufficientStockError
	switch {
	case err == nil:
		return h.reply(ctx, cmd, messaging.EvtPartsReserved, nil)
	case errors.As(err, &stockErr):
		return h.reply(ctx, cmd, messaging.EvtPartsReservationFailed,
			messaging.PartsReservationFailedPayload{Reason: err.Error(), PartID: stockErr.PartID})
	case errors.Is(err, domain.ErrInvalidReservation):
		return h.reply(ctx, cmd, messaging.EvtPartsReservationFailed,
			messaging.PartsReservationFailedPayload{Reason: err.Error()})
	default:
		return err
	}
}

func (h *CommandHandler) reply(ctx context.Context, cmd messaging.Envelope, eventType string, payload any) error {
	evt, err := messaging.ReplyTo(cmd, eventType, payload)
	if err != nil {
		return err
	}
	return h.publisher.Publish(ctx, evt)
}

func permanentIf(err error, permanent error) error {
	if errors.Is(err, permanent) {
		return fmt.Errorf("%w: %v", ErrPermanent, err)
	}
	return err
}
