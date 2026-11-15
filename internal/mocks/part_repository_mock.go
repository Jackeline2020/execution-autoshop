package mocks

import (
	"execution-autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type PartRepositoryMock struct {
	mock.Mock
}

func (m *PartRepositoryMock) Create(part domain.Part) (domain.Part, error) {
	args := m.Called(part)
	return args.Get(0).(domain.Part), args.Error(1)
}

func (m *PartRepositoryMock) FindAll() ([]domain.Part, error) {
	args := m.Called()
	return args.Get(0).([]domain.Part), args.Error(1)
}

func (m *PartRepositoryMock) FindByID(id string) (domain.Part, error) {
	args := m.Called(id)
	return args.Get(0).(domain.Part), args.Error(1)
}

func (m *PartRepositoryMock) FindLowStock() ([]domain.Part, error) {
	args := m.Called()
	return args.Get(0).([]domain.Part), args.Error(1)
}

func (m *PartRepositoryMock) Update(part domain.Part) error {
	args := m.Called(part)
	return args.Error(0)
}

func (m *PartRepositoryMock) UpdateStock(id string, quantity int) error {
	args := m.Called(id, quantity)
	return args.Error(0)
}

func (m *PartRepositoryMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
