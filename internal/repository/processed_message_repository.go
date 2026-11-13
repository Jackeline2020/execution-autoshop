package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ProcessedMessageRepository guarda os IDs das mensagens já processadas,
// para ignorar entregas repetidas do SQS.
type ProcessedMessageRepository interface {
	Exists(messageID string) (bool, error)
	MarkProcessed(messageID string) error
}

type ProcessedMessageDynamoRepository struct {
	client *dynamodb.Client
	table  string
	ttl    time.Duration
}

func NewProcessedMessageDynamoRepository(client *dynamodb.Client) *ProcessedMessageDynamoRepository {
	return &ProcessedMessageDynamoRepository{
		client: client,
		table:  tableName("PROCESSED_MESSAGES_TABLE", "processed_messages"),
		ttl:    7 * 24 * time.Hour,
	}
}

func (r *ProcessedMessageDynamoRepository) Exists(messageID string) (bool, error) {
	out, err := r.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            map[string]types.AttributeValue{"message_id": &types.AttributeValueMemberS{Value: messageID}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return false, err
	}
	return out.Item != nil, nil
}

// MarkProcessed grava o ID com prazo de expiração (expires_at), usado pelo
// TTL do DynamoDB para limpar registros antigos.
func (r *ProcessedMessageDynamoRepository) MarkProcessed(messageID string) error {
	now := time.Now()
	_, err := r.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item: map[string]types.AttributeValue{
			"message_id":   &types.AttributeValueMemberS{Value: messageID},
			"processed_at": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
			"expires_at":   &types.AttributeValueMemberN{Value: strconv.FormatInt(now.Add(r.ttl).Unix(), 10)},
		},
	})
	return err
}
