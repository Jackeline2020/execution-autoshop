// Package config lê a configuração do serviço a partir de variáveis de
// ambiente.
package config

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

// Port devolve a porta HTTP do serviço (padrão 8080).
func Port() string {
	return getEnv("PORT", "8080")
}

// LoadAWSConfig carrega credenciais e região pela cadeia padrão do SDK
// (variáveis de ambiente, IRSA no Kubernetes, perfil local). A variável
// AWS_ENDPOINT_URL, lida nativamente pelo SDK, redireciona as chamadas para
// o LocalStack em desenvolvimento e homologação.
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(getEnv("AWS_REGION", "us-east-1")))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
