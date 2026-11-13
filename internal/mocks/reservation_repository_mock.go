package mocks

import (
	"execution-autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type ReservationRepositoryMock struct {
	mock.Mock
}

func (m *ReservationRepositoryMock) Reserve(res domain.Reservation) error {
	return m.Called(res).Error(0)
}

func (m *ReservationRepositoryMock) Release(orderID string) error {
	return m.Called(orderID).Error(0)
}

func (m *ReservationRepositoryMock) Consume(orderID string) error {
	return m.Called(orderID).Error(0)
}

func (m *ReservationRepositoryMock) FindByOrderID(orderID string) (domain.Reservation, error) {
	args := m.Called(orderID)
	return args.Get(0).(domain.Reservation), args.Error(1)
}
