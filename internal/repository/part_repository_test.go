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

// Testes de integração contra o DynamoDB do LocalStack. Só rodam quando
// AWS_ENDPOINT_URL está definida (docker compose ou pipeline).
func newPartRepo(t *testing.T) *repository.PartDynamoRepository {
	t.Helper()
	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL não definida: teste de integração ignorado")
	}
	cfg, err := config.LoadAWSConfig(context.Background())
	require.NoError(t, err)
	return repository.NewPartDynamoRepository(repository.NewDynamoClient(cfg))
}

func newTestPart(stock, minStock int) domain.Part {
	p := domain.NewPart("Filtro de óleo "+uuid.NewString()[:8], "teste", "unidade", 35.5, stock, minStock)
	p.ID = uuid.NewString()
	return p
}

func TestPartRepository_CreateAndFind(t *testing.T) {
	repo := newPartRepo(t)
	part := newTestPart(10, 2)

	_, err := repo.Create(part)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(part.ID) })

	found, err := repo.FindByID(part.ID)
	require.NoError(t, err)
	assert.Equal(t, part, found)

	all, err := repo.FindAll()
	require.NoError(t, err)
	assert.Contains(t, all, part)
}

func TestPartRepository_CreateDuplicate(t *testing.T) {
	repo := newPartRepo(t)
	part := newTestPart(1, 0)

	_, err := repo.Create(part)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(part.ID) })

	_, err = repo.Create(part)
	assert.Error(t, err)
}

func TestPartRepository_FindByIDNotFound(t *testing.T) {
	repo := newPartRepo(t)
	_, err := repo.FindByID(uuid.NewString())
	assert.ErrorIs(t, err, repository.ErrPartNotFound)
}

func TestPartRepository_FindLowStock(t *testing.T) {
	repo := newPartRepo(t)
	low := newTestPart(1, 5)
	ok := newTestPart(10, 5)
	for _, p := range []domain.Part{low, ok} {
		_, err := repo.Create(p)
		require.NoError(t, err)
		id := p.ID
		t.Cleanup(func() { _ = repo.Delete(id) })
	}

	result, err := repo.FindLowStock()
	require.NoError(t, err)
	assert.Contains(t, result, low)
	assert.NotContains(t, result, ok)
}

func TestPartRepository_UpdateAndUpdateStock(t *testing.T) {
	repo := newPartRepo(t)
	part := newTestPart(10, 2)
	_, err := repo.Create(part)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.Delete(part.ID) })

	part.Name = "Filtro de ar"
	part.Price = 49.9
	require.NoError(t, repo.Update(part))
	require.NoError(t, repo.UpdateStock(part.ID, 7))

	found, err := repo.FindByID(part.ID)
	require.NoError(t, err)
	assert.Equal(t, "Filtro de ar", found.Name)
	assert.Equal(t, 49.9, found.Price)
	assert.Equal(t, 7, found.Stock)
}

func TestPartRepository_UpdateNotFound(t *testing.T) {
	repo := newPartRepo(t)
	part := newTestPart(1, 0)
	assert.ErrorIs(t, repo.Update(part), repository.ErrPartNotFound)
	assert.ErrorIs(t, repo.UpdateStock(part.ID, 3), repository.ErrPartNotFound)
}

func TestPartRepository_Delete(t *testing.T) {
	repo := newPartRepo(t)
	part := newTestPart(1, 0)
	_, err := repo.Create(part)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(part.ID))
	_, err = repo.FindByID(part.ID)
	assert.ErrorIs(t, err, repository.ErrPartNotFound)
}
