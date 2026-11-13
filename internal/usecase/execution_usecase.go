package usecase

import (
	"errors"
	"sort"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/repository"
)

type ExecutionUseCase struct {
	repo         repository.ExecutionRepository
	reservations repository.ReservationRepository
}

func NewExecutionUseCase(r repository.ExecutionRepository, res repository.ReservationRepository) *ExecutionUseCase {
	return &ExecutionUseCase{repo: r, reservations: res}
}

// StartDiagnosis coloca a OS na fila de diagnóstico. Se a OS já estiver
// registrada (comando repetido), devolve o registro existente.
func (u *ExecutionUseCase) StartDiagnosis(orderID string) (domain.Execution, error) {
	e, err := domain.NewExecution(orderID)
	if err != nil {
		return domain.Execution{}, err
	}
	err = u.repo.Create(e)
	if errors.Is(err, domain.ErrExecutionExists) {
		return u.repo.FindByOrderID(orderID)
	}
	if err != nil {
		return domain.Execution{}, err
	}
	return e, nil
}

func (u *ExecutionUseCase) RegisterDiagnosis(orderID, diagnosis string) (domain.Execution, error) {
	return u.change(orderID, func(e *domain.Execution) error { return e.RegisterDiagnosis(diagnosis) })
}

// Enqueue coloca a OS (já paga) na fila de reparo.
func (u *ExecutionUseCase) Enqueue(orderID string) (domain.Execution, error) {
	return u.change(orderID, (*domain.Execution).Enqueue)
}

func (u *ExecutionUseCase) Start(orderID string) (domain.Execution, error) {
	return u.change(orderID, (*domain.Execution).Start)
}

// Finish encerra o reparo e confirma o uso das peças reservadas. OS sem
// peças não têm reserva, e isso não impede a finalização.
func (u *ExecutionUseCase) Finish(orderID string) (domain.Execution, error) {
	return u.change(orderID, func(e *domain.Execution) error {
		if err := e.Finish(); err != nil {
			return err
		}
		err := u.reservations.Consume(orderID)
		if errors.Is(err, domain.ErrReservationNotFound) {
			return nil
		}
		return err
	})
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
	previous := e.Status
	if err := apply(&e); err != nil {
		return domain.Execution{}, err
	}
	if err := u.repo.Update(e, previous); err != nil {
		return domain.Execution{}, err
	}
	return e, nil
}
