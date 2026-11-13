package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPortDefault(t *testing.T) {
	t.Setenv("PORT", "")
	assert.Equal(t, "8080", Port())
}

func TestPortFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	assert.Equal(t, "9090", Port())
}

func TestLoadAWSConfigRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "sa-east-1")
	cfg, err := LoadAWSConfig(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "sa-east-1", cfg.Region)
}

func TestLoadAWSConfigDefaultRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	cfg, err := LoadAWSConfig(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "us-east-1", cfg.Region)
}
