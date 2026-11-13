package domain_test

import (
	"testing"

	"execution-autoshop/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecution_FullFlow(t *testing.T) {
	e, err := domain.NewExecution("os-1")
	require.NoError(t, err)
	assert.Equal(t, domain.ExecutionAwaitingDiagnosis, e.Status)
	assert.Equal(t, e.CreatedAt, e.WaitingSince())

	require.NoError(t, e.RegisterDiagnosis("  Troca de pastilhas de freio  "))
	assert.Equal(t, "Troca de pastilhas de freio", e.Diagnosis)
	assert.NotEmpty(t, e.DiagnosedAt)

	require.NoError(t, e.Enqueue())
	assert.Equal(t, domain.ExecutionQueued, e.Status)
	assert.Equal(t, e.QueuedAt, e.WaitingSince())

	require.NoError(t, e.Start())
	assert.NotEmpty(t, e.StartedAt)

	require.NoError(t, e.Finish())
	assert.Equal(t, domain.ExecutionFinished, e.Status)
	assert.NotEmpty(t, e.FinishedAt)
}

func TestNewExecution_RequiresOrder(t *testing.T) {
	_, err := domain.NewExecution("")
	assert.Error(t, err)
}

func TestExecution_EmptyDiagnosis(t *testing.T) {
	e, _ := domain.NewExecution("os-1")
	assert.Error(t, e.RegisterDiagnosis("   "))
	assert.Equal(t, domain.ExecutionAwaitingDiagnosis, e.Status)
}

func TestExecution_InvalidTransitions(t *testing.T) {
	e, _ := domain.NewExecution("os-1")

	var transErr *domain.InvalidTransitionError
	assert.ErrorAs(t, e.Start(), &transErr)
	assert.Equal(t, domain.ExecutionAwaitingDiagnosis, transErr.From)
	assert.Equal(t, domain.ExecutionInRepair, transErr.To)
	assert.Contains(t, transErr.Error(), "aguardando_diagnostico")

	assert.Error(t, e.Enqueue())
	assert.Error(t, e.Finish())

	require.NoError(t, e.RegisterDiagnosis("ok"))
	assert.Error(t, e.RegisterDiagnosis("de novo"))
}
