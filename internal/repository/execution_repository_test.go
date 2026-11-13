package repository_test

import (
	"context"
	"os"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/repository"
	"execution-autoshop/pkg/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newExecutionRepo(t *testing.T) *repository.ExecutionDynamoRepository {
	t.Helper()
	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL não definida: teste de integração ignorado")
	}
	cfg, err := config.LoadAWSConfig(context.Background())
	require.NoError(t, err)
	return repository.NewExecutionDynamoRepository(repository.NewDynamoClient(cfg))
}

func newExecution(t *testing.T) domain.Execution {
	t.Helper()
	e, err := domain.NewExecution(uuid.NewString())
	require.NoError(t, err)
	return e
}

func TestExecutionRepository_CreateFindAndDuplicate(t *testing.T) {
	repo := newExecutionRepo(t)
	e := newExecution(t)

	require.NoError(t, repo.Create(e))
	assert.ErrorIs(t, repo.Create(e), domain.ErrExecutionExists)

	found, err := repo.FindByOrderID(e.OrderID)
	require.NoError(t, err)
	assert.Equal(t, e, found)
}

func TestExecutionRepository_NotFound(t *testing.T) {
	repo := newExecutionRepo(t)
	_, err := repo.FindByOrderID(uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrExecutionNotFound)
}

func TestExecutionRepository_UpdateWithStatusCheck(t *testing.T) {
	repo := newExecutionRepo(t)
	e := newExecution(t)
	require.NoError(t, repo.Create(e))

	stale := e
	require.NoError(t, e.RegisterDiagnosis("Troca de óleo"))
	require.NoError(t, repo.Update(e, domain.ExecutionAwaitingDiagnosis))

	// Uma segunda operação baseada na leitura antiga é recusada
	require.NoError(t, stale.RegisterDiagnosis("Outro diagnóstico"))
	assert.ErrorIs(t, repo.Update(stale, domain.ExecutionAwaitingDiagnosis), repository.ErrConcurrentUpdate)

	found, err := repo.FindByOrderID(e.OrderID)
	require.NoError(t, err)
	assert.Equal(t, "Troca de óleo", found.Diagnosis)
}

func TestExecutionRepository_FindByStatuses(t *testing.T) {
	repo := newExecutionRepo(t)
	waiting := newExecution(t)
	diagnosed := newExecution(t)
	require.NoError(t, repo.Create(waiting))
	require.NoError(t, repo.Create(diagnosed))
	require.NoError(t, diagnosed.RegisterDiagnosis("ok"))
	require.NoError(t, repo.Update(diagnosed, domain.ExecutionAwaitingDiagnosis))

	result, err := repo.FindByStatuses(domain.ExecutionAwaitingDiagnosis, domain.ExecutionQueued)
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, r := range result {
		ids[r.OrderID] = true
	}
	assert.True(t, ids[waiting.OrderID])
	assert.False(t, ids[diagnosed.OrderID])
}
