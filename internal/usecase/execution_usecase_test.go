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

func setupExecution() (*usecase.ExecutionUseCase, *mocks.ExecutionRepositoryMock, *mocks.ReservationRepositoryMock) {
	repo := new(mocks.ExecutionRepositoryMock)
	res := new(mocks.ReservationRepositoryMock)
	return usecase.NewExecutionUseCase(repo, res), repo, res
}

func executionIn(status domain.ExecutionStatus) domain.Execution {
	return domain.Execution{OrderID: "os-1", Status: status, CreatedAt: "2026-01-01T10:00:00Z"}
}

func TestStartDiagnosis_Creates(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("Create", mock.AnythingOfType("domain.Execution")).Return(nil)

	e, err := uc.StartDiagnosis("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionAwaitingDiagnosis, e.Status)
}

func TestStartDiagnosis_RepeatedReturnsExisting(t *testing.T) {
	uc, repo, _ := setupExecution()
	existing := executionIn(domain.ExecutionDiagnosed)
	repo.On("Create", mock.Anything).Return(domain.ErrExecutionExists)
	repo.On("FindByOrderID", "os-1").Return(existing, nil)

	e, err := uc.StartDiagnosis("os-1")
	require.NoError(t, err)
	assert.Equal(t, existing, e)
}

func TestStartDiagnosis_Errors(t *testing.T) {
	uc, repo, _ := setupExecution()
	_, err := uc.StartDiagnosis("")
	assert.ErrorIs(t, err, domain.ErrOrderRequired)

	repo.On("Create", mock.Anything).Return(errors.New("dynamo fora"))
	_, err = uc.StartDiagnosis("os-1")
	assert.Error(t, err)
}

func TestRegisterDiagnosis(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	repo.On("Update", mock.Anything, domain.ExecutionAwaitingDiagnosis).Return(nil)

	e, err := uc.RegisterDiagnosis("os-1", "Troca de óleo")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionDiagnosed, e.Status)
	assert.Equal(t, "Troca de óleo", e.Diagnosis)
}

func TestEnqueueAndStart(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionDiagnosed), nil).Once()
	repo.On("Update", mock.Anything, domain.ExecutionDiagnosed).Return(nil)
	e, err := uc.Enqueue("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionQueued, e.Status)

	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionQueued), nil).Once()
	repo.On("Update", mock.Anything, domain.ExecutionQueued).Return(nil)
	e, err = uc.Start("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionInRepair, e.Status)
}

func TestFinish_ConsumesReservation(t *testing.T) {
	uc, repo, res := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	repo.On("Update", mock.Anything, domain.ExecutionInRepair).Return(nil)
	res.On("Consume", "os-1").Return(nil)

	e, err := uc.Finish("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionFinished, e.Status)
	res.AssertExpectations(t)
}

func TestFinish_WithoutReservation(t *testing.T) {
	uc, repo, res := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	repo.On("Update", mock.Anything, domain.ExecutionInRepair).Return(nil)
	res.On("Consume", "os-1").Return(domain.ErrReservationNotFound)

	_, err := uc.Finish("os-1")
	assert.NoError(t, err)
}

func TestFinish_ConsumeFailsDoesNotSave(t *testing.T) {
	uc, repo, res := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	res.On("Consume", "os-1").Return(errors.New("dynamo fora"))

	_, err := uc.Finish("os-1")
	assert.Error(t, err)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestChange_Errors(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("FindByOrderID", "nao-existe").Return(domain.Execution{}, domain.ErrExecutionNotFound)
	_, err := uc.Start("nao-existe")
	assert.ErrorIs(t, err, domain.ErrExecutionNotFound)

	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	var transErr *domain.InvalidTransitionError
	_, err = uc.Finish("os-1")
	assert.ErrorAs(t, err, &transErr)

	repo.On("Update", mock.Anything, mock.Anything).Return(errors.New("conflito"))
	_, err = uc.RegisterDiagnosis("os-1", "ok")
	assert.Error(t, err)
}

func TestQueue_OrdersByWaitingTime(t *testing.T) {
	uc, repo, _ := setupExecution()
	newer := domain.Execution{OrderID: "a", Status: domain.ExecutionAwaitingDiagnosis, CreatedAt: "2026-01-01T12:00:00Z"}
	older := domain.Execution{OrderID: "b", Status: domain.ExecutionQueued, CreatedAt: "2026-01-01T08:00:00Z", QueuedAt: "2026-01-01T09:00:00Z"}
	repo.On("FindByStatuses", []domain.ExecutionStatus{domain.ExecutionAwaitingDiagnosis, domain.ExecutionQueued}).
		Return([]domain.Execution{newer, older}, nil)

	items, err := uc.Queue()
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, []string{items[0].OrderID, items[1].OrderID})
}

func TestQueue_Error(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("FindByStatuses", mock.Anything).Return([]domain.Execution(nil), errors.New("dynamo fora"))
	_, err := uc.Queue()
	assert.Error(t, err)
}

func TestGetByOrderID(t *testing.T) {
	uc, repo, _ := setupExecution()
	repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionQueued), nil)
	e, err := uc.GetByOrderID("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionQueued, e.Status)
}
