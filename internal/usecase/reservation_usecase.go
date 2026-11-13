package usecase

import (
	"errors"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/repository"
)

type ReservationUseCase struct {
	repo repository.ReservationRepository
}

func NewReservationUseCase(r repository.ReservationRepository) *ReservationUseCase {
	return &ReservationUseCase{repo: r}
}

// Reserve separa as peças da OS. Se a OS já tiver reserva (comando
// repetido), devolve a reserva existente em vez de descontar o estoque de
// novo.
func (u *ReservationUseCase) Reserve(orderID string, items []domain.ReservationItem) (domain.Reservation, error) {
	res, err := domain.NewReservation(orderID, items)
	if err != nil {
		return domain.Reservation{}, err
	}

	err = u.repo.Reserve(res)
	if errors.Is(err, domain.ErrReservationExists) {
		return u.repo.FindByOrderID(orderID)
	}
	if err != nil {
		return domain.Reservation{}, err
	}
	return res, nil
}

// Release é a compensação da reserva: devolve o estoque.
func (u *ReservationUseCase) Release(orderID string) error {
	return u.repo.Release(orderID)
}

// Consume confirma o uso das peças no reparo.
func (u *ReservationUseCase) Consume(orderID string) error {
	return u.repo.Consume(orderID)
}

func (u *ReservationUseCase) GetByOrderID(orderID string) (domain.Reservation, error) {
	return u.repo.FindByOrderID(orderID)
}
