package repository_test

import (
	"context"
	"os"
	"testing"

	"execution-autoshop/internal/repository"
	"execution-autoshop/pkg/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessedMessageRepository(t *testing.T) {
	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL não definida: teste de integração ignorado")
	}
	cfg, err := config.LoadAWSConfig(context.Background())
	require.NoError(t, err)
	repo := repository.NewProcessedMessageDynamoRepository(repository.NewDynamoClient(cfg))

	id := uuid.NewString()
	exists, err := repo.Exists(id)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, repo.MarkProcessed(id))
	exists, err = repo.Exists(id)
	require.NoError(t, err)
	assert.True(t, exists)
}
