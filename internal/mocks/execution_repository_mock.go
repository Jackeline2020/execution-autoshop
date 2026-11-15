package mocks

import (
	"execution-autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type ExecutionRepositoryMock struct {
	mock.Mock
}

func (m *ExecutionRepositoryMock) Create(e domain.Execution) error {
	return m.Called(e).Error(0)
}

func (m *ExecutionRepositoryMock) FindByOrderID(orderID string) (domain.Execution, error) {
	args := m.Called(orderID)
	return args.Get(0).(domain.Execution), args.Error(1)
}

func (m *ExecutionRepositoryMock) Update(e domain.Execution, previous domain.ExecutionStatus) error {
	return m.Called(e, previous).Error(0)
}

func (m *ExecutionRepositoryMock) FindByStatuses(statuses ...domain.ExecutionStatus) ([]domain.Execution, error) {
	args := m.Called(statuses)
	return args.Get(0).([]domain.Execution), args.Error(1)
}
