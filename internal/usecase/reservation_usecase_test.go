package usecase_test

import (
	"errors"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var reservationItems = []domain.ReservationItem{{PartID: "p1", Quantity: 2}}

func TestReserve_Success(t *testing.T) {
	repo := new(mocks.ReservationRepositoryMock)
	repo.On("Reserve", mock.AnythingOfType("domain.Reservation")).Return(nil)

	res, err := usecase.NewReservationUseCase(repo).Reserve("os-1", reservationItems)
	require.NoError(t, err)
	assert.Equal(t, "os-1", res.OrderID)
	assert.Equal(t, domain.ReservationReserved, res.Status)
	repo.AssertExpectations(t)
}

func TestReserve_InvalidItems(t *testing.T) {
	repo := new(mocks.ReservationRepositoryMock)
	_, err := usecase.NewReservationUseCase(repo).Reserve("os-1", nil)
	assert.Error(t, err)
	repo.AssertNotCalled(t, "Reserve", mock.Anything)
}

func TestReserve_AlreadyReservedReturnsExisting(t *testing.T) {
	existing := domain.Reservation{OrderID: "os-1", Items: reservationItems, Status: domain.ReservationReserved}
	repo := new(mocks.ReservationRepositoryMock)
	repo.On("Reserve", mock.Anything).Return(domain.ErrReservationExists)
	repo.On("FindByOrderID", "os-1").Return(existing, nil)

	res, err := usecase.NewReservationUseCase(repo).Reserve("os-1", reservationItems)
	require.NoError(t, err)
	assert.Equal(t, existing, res)
}

func TestReserve_InsufficientStock(t *testing.T) {
	repo := new(mocks.ReservationRepositoryMock)
	repo.On("Reserve", mock.Anything).Return(&domain.InsufficientStockError{PartID: "p1"})

	_, err := usecase.NewReservationUseCase(repo).Reserve("os-1", reservationItems)
	var stockErr *domain.InsufficientStockError
	assert.ErrorAs(t, err, &stockErr)
}

func TestReleaseConsumeAndGet(t *testing.T) {
	repo := new(mocks.ReservationRepositoryMock)
	repo.On("Release", "os-1").Return(nil)
	repo.On("Consume", "os-2").Return(errors.New("falhou"))
	repo.On("FindByOrderID", "os-3").Return(domain.Reservation{OrderID: "os-3"}, nil)
	uc := usecase.NewReservationUseCase(repo)

	assert.NoError(t, uc.Release("os-1"))
	assert.Error(t, uc.Consume("os-2"))
	res, err := uc.GetByOrderID("os-3")
	require.NoError(t, err)
	assert.Equal(t, "os-3", res.OrderID)
}
