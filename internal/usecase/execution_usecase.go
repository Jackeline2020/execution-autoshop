package usecase

import (
	"context"
	"errors"
	"sort"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/repository"
	"execution-autoshop/pkg/logger"
)

// EventPublisher envia eventos para o orquestrador da Saga.
type EventPublisher interface {
	Publish(ctx context.Context, env messaging.Envelope) error
}

type ExecutionUseCase struct {
	repo         repository.ExecutionRepository
	reservations repository.ReservationRepository
	publisher    EventPublisher
}

func NewExecutionUseCase(r repository.ExecutionRepository, res repository.ReservationRepository, pub EventPublisher) *ExecutionUseCase {
	return &ExecutionUseCase{repo: r, reservations: res, publisher: pub}
}

// StartDiagnosis coloca a OS na fila de diagnóstico. Se a OS já estiver
// registrada (comando repetido), devolve o registro existente.
func (u *ExecutionUseCase) StartDiagnosis(orderID, sagaID, correlationID string) (domain.Execution, error) {
	e, err := domain.NewExecution(orderID)
	if err != nil {
		return domain.Execution{}, err
	}
	e.SagaID = sagaID
	e.CorrelationID = correlationID

	err = u.repo.Create(e)
	if errors.Is(err, domain.ErrExecutionExists) {
		return u.repo.FindByOrderID(orderID)
	}
	if err != nil {
		return domain.Execution{}, err
	}
	return e, nil
}

// RegisterDiagnosis grava o diagnóstico do mecânico e avisa o orquestrador,
// que segue para o orçamento.
func (u *ExecutionUseCase) RegisterDiagnosis(orderID, diagnosis string) (domain.Execution, error) {
	e, err := u.change(orderID, func(e *domain.Execution) error { return e.RegisterDiagnosis(diagnosis) })
	if err == nil {
		u.publish(e, messaging.EvtDiagnosisFinished, messaging.DiagnosisFinishedPayload{Diagnosis: e.Diagnosis})
	}
	return e, err
}

// Enqueue coloca a OS (já paga) na fila de reparo. Se ela já estiver na
// fila ou adiante (comando repetido), não faz nada.
func (u *ExecutionUseCase) Enqueue(orderID string) (domain.Execution, error) {
	e, err := u.repo.FindByOrderID(orderID)
	if err != nil {
		return domain.Execution{}, err
	}
	switch e.Status {
	case domain.ExecutionQueued, domain.ExecutionInRepair, domain.ExecutionFinished:
		return e, nil
	}
	return u.save(e, (*domain.Execution).Enqueue)
}

func (u *ExecutionUseCase) Start(orderID string) (domain.Execution, error) {
	e, err := u.change(orderID, (*domain.Execution).Start)
	if err == nil {
		u.publish(e, messaging.EvtExecutionStarted, nil)
	}
	return e, err
}

// Finish encerra o reparo e confirma o uso das peças reservadas. OS sem
// peças não têm reserva, e isso não impede a finalização.
func (u *ExecutionUseCase) Finish(orderID string) (domain.Execution, error) {
	e, err := u.change(orderID, func(e *domain.Execution) error {
		if err := e.Finish(); err != nil {
			return err
		}
		err := u.reservations.Consume(orderID)
		if errors.Is(err, domain.ErrReservationNotFound) {
			return nil
		}
		return err
	})
	if err == nil {
		u.publish(e, messaging.EvtExecutionFinished, nil)
	}
	return e, err
}

func (u *ExecutionUseCase) GetByOrderID(orderID string) (domain.Execution, error) {
	return u.repo.FindByOrderID(orderID)
}

// Queue devolve as OS que aguardam a oficina (diagnóstico ou reparo), das
// que esperam há mais tempo para as mais recentes.
func (u *ExecutionUseCase) Queue() ([]domain.Execution, error) {
	items, err := u.repo.FindByStatuses(domain.ExecutionAwaitingDiagnosis, domain.ExecutionQueued)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].WaitingSince() < items[j].WaitingSince() })
	return items, nil
}

func (u *ExecutionUseCase) change(orderID string, apply func(*domain.Execution) error) (domain.Execution, error) {
	e, err := u.repo.FindByOrderID(orderID)
	if err != nil {
		return domain.Execution{}, err
	}
	return u.save(e, apply)
}

// save aplica a mudança e grava, desde que o status no banco não tenha
// mudado desde a leitura.
func (u *ExecutionUseCase) save(e domain.Execution, apply func(*domain.Execution) error) (domain.Execution, error) {
	previous := e.Status
	if err := apply(&e); err != nil {
		return domain.Execution{}, err
	}
	if err := u.repo.Update(e, previous); err != nil {
		return domain.Execution{}, err
	}
	return e, nil
}

// publish avisa o orquestrador sobre uma ação do mecânico. A mudança já foi
// gravada; se o envio falhar após as tentativas do publicador, a falha é
// registrada no log e o prazo do passo no orquestrador trata o caso.
func (u *ExecutionUseCase) publish(e domain.Execution, eventType string, payload any) {
	env, err := messaging.NewEvent(eventType, e.SagaID, e.OrderID, e.CorrelationID, payload)
	if err == nil {
		err = u.publisher.Publish(context.Background(), env)
	}
	if err != nil {
		logger.Error("falha ao publicar evento", "type", eventType, "order_id", e.OrderID,
			"correlation_id", e.CorrelationID, "error", err.Error())
		return
	}
	logger.Info("evento publicado", "type", eventType, "order_id", e.OrderID, "correlation_id", e.CorrelationID)
}
