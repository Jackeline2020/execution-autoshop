package usecase_test

import (
	"errors"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type executionMocks struct {
	repo *mocks.ExecutionRepositoryMock
	res  *mocks.ReservationRepositoryMock
	pub  *mocks.PublisherMock
}

func setupExecution() (*usecase.ExecutionUseCase, executionMocks) {
	m := executionMocks{
		repo: new(mocks.ExecutionRepositoryMock),
		res:  new(mocks.ReservationRepositoryMock),
		pub:  new(mocks.PublisherMock),
	}
	return usecase.NewExecutionUseCase(m.repo, m.res, m.pub), m
}

func executionIn(status domain.ExecutionStatus) domain.Execution {
	return domain.Execution{OrderID: "os-1", SagaID: "saga-1", CorrelationID: "corr-1", Status: status, CreatedAt: "2026-01-01T10:00:00Z"}
}

func TestStartDiagnosis_CreatesWithSagaContext(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("Create", mock.AnythingOfType("domain.Execution")).Return(nil)

	e, err := uc.StartDiagnosis("os-1", "saga-1", "corr-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionAwaitingDiagnosis, e.Status)
	assert.Equal(t, "saga-1", e.SagaID)
	assert.Equal(t, "corr-1", e.CorrelationID)
	assert.Empty(t, m.pub.Published)
}

func TestStartDiagnosis_RepeatedReturnsExisting(t *testing.T) {
	uc, m := setupExecution()
	existing := executionIn(domain.ExecutionDiagnosed)
	m.repo.On("Create", mock.Anything).Return(domain.ErrExecutionExists)
	m.repo.On("FindByOrderID", "os-1").Return(existing, nil)

	e, err := uc.StartDiagnosis("os-1", "saga-1", "corr-1")
	require.NoError(t, err)
	assert.Equal(t, existing, e)
}

func TestStartDiagnosis_Errors(t *testing.T) {
	uc, m := setupExecution()
	_, err := uc.StartDiagnosis("", "s", "c")
	assert.ErrorIs(t, err, domain.ErrOrderRequired)

	m.repo.On("Create", mock.Anything).Return(errors.New("dynamo fora"))
	_, err = uc.StartDiagnosis("os-1", "s", "c")
	assert.Error(t, err)
}

func TestRegisterDiagnosis_PublishesEvent(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionAwaitingDiagnosis).Return(nil)
	m.pub.On("Publish", messaging.EvtDiagnosisFinished, "os-1").Return(nil)

	e, err := uc.RegisterDiagnosis("os-1", "Troca de óleo")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionDiagnosed, e.Status)

	require.Len(t, m.pub.Published, 1)
	evt := m.pub.Published[0]
	assert.Equal(t, "saga-1", evt.SagaID)
	assert.Equal(t, "corr-1", evt.CorrelationID)
	var p messaging.DiagnosisFinishedPayload
	require.NoError(t, evt.DecodePayload(&p))
	assert.Equal(t, "Troca de óleo", p.Diagnosis)
}

func TestRegisterDiagnosis_PublishFailureKeepsChange(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionAwaitingDiagnosis).Return(nil)
	m.pub.On("Publish", mock.Anything, mock.Anything).Return(errors.New("sqs fora"))

	e, err := uc.RegisterDiagnosis("os-1", "Troca de óleo")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionDiagnosed, e.Status)
}

func TestEnqueue(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionDiagnosed), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionDiagnosed).Return(nil)

	e, err := uc.Enqueue("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionQueued, e.Status)
}

func TestEnqueue_AlreadyQueuedIsIgnored(t *testing.T) {
	for _, status := range []domain.ExecutionStatus{domain.ExecutionQueued, domain.ExecutionInRepair, domain.ExecutionFinished} {
		uc, m := setupExecution()
		m.repo.On("FindByOrderID", "os-1").Return(executionIn(status), nil)

		e, err := uc.Enqueue("os-1")
		require.NoError(t, err)
		assert.Equal(t, status, e.Status)
		m.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	}
}

func TestEnqueue_Errors(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "x").Return(domain.Execution{}, domain.ErrExecutionNotFound)
	_, err := uc.Enqueue("x")
	assert.ErrorIs(t, err, domain.ErrExecutionNotFound)

	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	var transErr *domain.InvalidTransitionError
	_, err = uc.Enqueue("os-1")
	assert.ErrorAs(t, err, &transErr)
}

func TestStart_PublishesEvent(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionQueued), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionQueued).Return(nil)
	m.pub.On("Publish", messaging.EvtExecutionStarted, "os-1").Return(nil)

	e, err := uc.Start("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionInRepair, e.Status)
	m.pub.AssertExpectations(t)
}

func TestFinish_ConsumesReservationAndPublishes(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionInRepair).Return(nil)
	m.res.On("Consume", "os-1").Return(nil)
	m.pub.On("Publish", messaging.EvtExecutionFinished, "os-1").Return(nil)

	e, err := uc.Finish("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionFinished, e.Status)
	m.res.AssertExpectations(t)
	m.pub.AssertExpectations(t)
}

func TestFinish_WithoutReservation(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	m.repo.On("Update", mock.Anything, domain.ExecutionInRepair).Return(nil)
	m.res.On("Consume", "os-1").Return(domain.ErrReservationNotFound)
	m.pub.On("Publish", mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Finish("os-1")
	assert.NoError(t, err)
}

func TestFinish_ConsumeFailsDoesNotSave(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionInRepair), nil)
	m.res.On("Consume", "os-1").Return(errors.New("dynamo fora"))

	_, err := uc.Finish("os-1")
	assert.Error(t, err)
	m.repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	assert.Empty(t, m.pub.Published)
}

func TestChange_Errors(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "nao-existe").Return(domain.Execution{}, domain.ErrExecutionNotFound)
	_, err := uc.Start("nao-existe")
	assert.ErrorIs(t, err, domain.ErrExecutionNotFound)

	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionAwaitingDiagnosis), nil)
	var transErr *domain.InvalidTransitionError
	_, err = uc.Finish("os-1")
	assert.ErrorAs(t, err, &transErr)

	m.repo.On("Update", mock.Anything, mock.Anything).Return(errors.New("conflito"))
	_, err = uc.RegisterDiagnosis("os-1", "ok")
	assert.Error(t, err)
	assert.Empty(t, m.pub.Published)
}

func TestQueue_OrdersByWaitingTime(t *testing.T) {
	uc, m := setupExecution()
	newer := domain.Execution{OrderID: "a", Status: domain.ExecutionAwaitingDiagnosis, CreatedAt: "2026-01-01T12:00:00Z"}
	older := domain.Execution{OrderID: "b", Status: domain.ExecutionQueued, CreatedAt: "2026-01-01T08:00:00Z", QueuedAt: "2026-01-01T09:00:00Z"}
	m.repo.On("FindByStatuses", []domain.ExecutionStatus{domain.ExecutionAwaitingDiagnosis, domain.ExecutionQueued}).
		Return([]domain.Execution{newer, older}, nil)

	items, err := uc.Queue()
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, []string{items[0].OrderID, items[1].OrderID})
}

func TestQueue_Error(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByStatuses", mock.Anything).Return([]domain.Execution(nil), errors.New("dynamo fora"))
	_, err := uc.Queue()
	assert.Error(t, err)
}

func TestGetByOrderID(t *testing.T) {
	uc, m := setupExecution()
	m.repo.On("FindByOrderID", "os-1").Return(executionIn(domain.ExecutionQueued), nil)
	e, err := uc.GetByOrderID("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionQueued, e.Status)
}
