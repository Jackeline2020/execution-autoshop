package auth_test

import (
	"execution-autoshop/pkg/auth"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := auth.GenerateToken("user-1", "admin")

	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestGenerateToken_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := auth.GenerateToken("user-1", "admin")

	assert.Error(t, err)
}

func TestValidateToken_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := auth.GenerateToken("user-1", "admin")
	assert.NoError(t, err)

	claims, err := auth.ValidateToken(token)

	assert.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
}

func TestValidateToken_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := auth.GenerateToken("user-1", "admin")
	assert.NoError(t, err)

	t.Setenv("JWT_SECRET", "")
	_, err = auth.ValidateToken(token)

	assert.Error(t, err)
}

func TestValidateToken_InvalidToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	_, err := auth.ValidateToken("isso-nao-e-um-jwt")

	assert.Error(t, err)
}

func TestValidateToken_WrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "segredo-1")
	token, err := auth.GenerateToken("user-1", "admin")
	assert.NoError(t, err)

	t.Setenv("JWT_SECRET", "segredo-2")
	_, err = auth.ValidateToken(token)

	assert.Error(t, err)
}
