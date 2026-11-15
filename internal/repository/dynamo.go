package repository

import (
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewDynamoClient cria o cliente DynamoDB a partir da configuração da AWS.
// Em desenvolvimento, AWS_ENDPOINT_URL aponta para o LocalStack.
func NewDynamoClient(cfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg)
}

// tableName permite trocar o nome da tabela por variável de ambiente
// (ex.: prefixo por ambiente na AWS), mantendo o nome padrão no local.
func tableName(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}
