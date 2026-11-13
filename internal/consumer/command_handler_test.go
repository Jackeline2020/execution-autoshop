package consumer_test

import (
	"context"
	"errors"
	"testing"

	"execution-autoshop/internal/consumer"
	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type handlerMocks struct {
	executions   *mocks.ExecutionRepositoryMock
	reservations *mocks.ReservationRepositoryMock
	pub          *mocks.PublisherMock
}

func setupHandler() (*consumer.CommandHandler, handlerMocks) {
	m := handlerMocks{
		executions:   new(mocks.ExecutionRepositoryMock),
		reservations: new(mocks.ReservationRepositoryMock),
		pub:          new(mocks.PublisherMock),
	}
	execUC := usecase.NewExecutionUseCase(m.executions, m.reservations, m.pub)
	resUC := usecase.NewReservationUseCase(m.reservations)
	return consumer.NewCommandHandler(execUC, resUC, m.pub), m
}

func command(t *testing.T, cmdType string, payload any) messaging.Envelope {
	t.Helper()
	env, err := messaging.NewEvent(cmdType, "saga-1", "os-1", "corr-1", payload)
	require.NoError(t, err)
	return env
}

func TestHandle_StartDiagnosis(t *testing.T) {
	h, m := setupHandler()
	m.executions.On("Create", mock.MatchedBy(func(e domain.Execution) bool {
		return e.OrderID == "os-1" && e.SagaID == "saga-1" && e.CorrelationID == "corr-1"
	})).Return(nil)

	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdStartDiagnosis, nil)))
	assert.Empty(t, m.pub.Published)
}

func TestHandle_StartDiagnosisWithoutOrderIsPermanent(t *testing.T) {
	h, _ := setupHandler()
	cmd := command(t, messaging.CmdStartDiagnosis, nil)
	cmd.OrderID = ""
	assert.ErrorIs(t, h.Handle(context.Background(), cmd), consumer.ErrPermanent)
}

func TestHandle_ReservePartsSuccess(t *testing.T) {
	h, m := setupHandler()
	m.reservations.On("Reserve", mock.Anything).Return(nil)
	m.pub.On("Publish", messaging.EvtPartsReserved, "os-1").Return(nil)

	payload := messaging.ReservePartsPayload{Items: []domain.ReservationItem{{PartID: "p1", Quantity: 2}}}
	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdReserveParts, payload)))

	require.Len(t, m.pub.Published, 1)
	assert.Equal(t, "saga-1", m.pub.Published[0].SagaID)
	assert.Equal(t, "corr-1", m.pub.Published[0].CorrelationID)
}

func TestHandle_ReservePartsInsufficientStock(t *testing.T) {
	h, m := setupHandler()
	m.reservations.On("Reserve", mock.Anything).Return(&domain.InsufficientStockError{PartID: "p1"})
	m.pub.On("Publish", messaging.EvtPartsReservationFailed, "os-1").Return(nil)

	payload := messaging.ReservePartsPayload{Items: []domain.ReservationItem{{PartID: "p1", Quantity: 2}}}
	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdReserveParts, payload)))

	var p messaging.PartsReservationFailedPayload
	require.NoError(t, m.pub.Published[0].DecodePayload(&p))
	assert.Equal(t, "p1", p.PartID)
}

func TestHandle_ReservePartsInvalidCommand(t *testing.T) {
	h, m := setupHandler()
	m.pub.On("Publish", messaging.EvtPartsReservationFailed, "os-1").Return(nil)

	// sem payload
	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdReserveParts, nil)))
	// sem itens
	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdReserveParts, messaging.ReservePartsPayload{})))
	assert.Len(t, m.pub.Published, 2)
	m.reservations.AssertNotCalled(t, "Reserve", mock.Anything)
}

func TestHandle_ReservePartsTechnicalErrorIsRetried(t *testing.T) {
	h, m := setupHandler()
	m.reservations.On("Reserve", mock.Anything).Return(errors.New("dynamo fora"))

	payload := messaging.ReservePartsPayload{Items: []domain.ReservationItem{{PartID: "p1", Quantity: 1}}}
	err := h.Handle(context.Background(), command(t, messaging.CmdReserveParts, payload))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, consumer.ErrPermanent)
	assert.Empty(t, m.pub.Published)
}

func TestHandle_ReleaseParts(t *testing.T) {
	h, m := setupHandler()
	m.reservations.On("Release", "os-1").Return(nil)
	m.pub.On("Publish", messaging.EvtPartsReleased, "os-1").Return(nil)

	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdReleaseParts, nil)))
	m.pub.AssertExpectations(t)
}

func TestHandle_ReleasePartsError(t *testing.T) {
	h, m := setupHandler()
	m.reservations.On("Release", "os-1").Return(errors.New("dynamo fora"))
	assert.Error(t, h.Handle(context.Background(), command(t, messaging.CmdReleaseParts, nil)))
}

func TestHandle_StartExecution(t *testing.T) {
	h, m := setupHandler()
	m.executions.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionDiagnosed}, nil)
	m.executions.On("Update", mock.Anything, domain.ExecutionDiagnosed).Return(nil)
	m.pub.On("Publish", messaging.EvtExecutionQueued, "os-1").Return(nil)

	require.NoError(t, h.Handle(context.Background(), command(t, messaging.CmdStartExecution, nil)))
	m.pub.AssertExpectations(t)
}

func TestHandle_StartExecutionPermanentErrors(t *testing.T) {
	h, m := setupHandler()
	m.executions.On("FindByOrderID", "os-1").Return(domain.Execution{}, domain.ErrExecutionNotFound).Once()
	assert.ErrorIs(t, h.Handle(context.Background(), command(t, messaging.CmdStartExecution, nil)), consumer.ErrPermanent)

	m.executions.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionAwaitingDiagnosis}, nil).Once()
	assert.ErrorIs(t, h.Handle(context.Background(), command(t, messaging.CmdStartExecution, nil)), consumer.ErrPermanent)
}

func TestHandle_StartExecutionTechnicalError(t *testing.T) {
	h, m := setupHandler()
	m.executions.On("FindByOrderID", "os-1").Return(domain.Execution{}, errors.New("dynamo fora"))
	err := h.Handle(context.Background(), command(t, messaging.CmdStartExecution, nil))
	assert.Error(t, err)
	assert.NotErrorIs(t, err, consumer.ErrPermanent)
}

func TestHandle_UnknownCommand(t *testing.T) {
	h, _ := setupHandler()
	assert.ErrorIs(t, h.Handle(context.Background(), command(t, "Desconhecido", nil)), consumer.ErrPermanent)
}
