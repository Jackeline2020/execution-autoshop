package repository

import (
	"context"
	"errors"
	"fmt"

	"execution-autoshop/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ErrConcurrentUpdate indica que a execução mudou de status entre a
// leitura e a gravação (ex.: dois cliques em "iniciar" ao mesmo tempo).
var ErrConcurrentUpdate = errors.New("a execução foi alterada por outra operação; tente novamente")

type ExecutionRepository interface {
	Create(e domain.Execution) error
	FindByOrderID(orderID string) (domain.Execution, error)
	Update(e domain.Execution, previous domain.ExecutionStatus) error
	FindByStatuses(statuses ...domain.ExecutionStatus) ([]domain.Execution, error)
}

type ExecutionDynamoRepository struct {
	client *dynamodb.Client
	table  string
}

func NewExecutionDynamoRepository(client *dynamodb.Client) *ExecutionDynamoRepository {
	return &ExecutionDynamoRepository{client: client, table: tableName("EXECUTIONS_TABLE", "executions")}
}

func (r *ExecutionDynamoRepository) Create(e domain.Execution) error {
	item, err := attributevalue.MarshalMap(e)
	if err != nil {
		return fmt.Errorf("erro ao converter execução: %w", err)
	}
	_, err = r.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName:           aws.String(r.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(order_id)"),
	})
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return domain.ErrExecutionExists
	}
	return err
}

func (r *ExecutionDynamoRepository) FindByOrderID(orderID string) (domain.Execution, error) {
	out, err := r.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            map[string]types.AttributeValue{"order_id": &types.AttributeValueMemberS{Value: orderID}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return domain.Execution{}, err
	}
	if out.Item == nil {
		return domain.Execution{}, domain.ErrExecutionNotFound
	}

	var e domain.Execution
	if err := attributevalue.UnmarshalMap(out.Item, &e); err != nil {
		return domain.Execution{}, fmt.Errorf("erro ao ler execução: %w", err)
	}
	return e, nil
}

// Update grava a execução somente se o status no banco ainda for o
// anterior, evitando que duas operações simultâneas sobrescrevam uma à
// outra.
func (r *ExecutionDynamoRepository) Update(e domain.Execution, previous domain.ExecutionStatus) error {
	item, err := attributevalue.MarshalMap(e)
	if err != nil {
		return fmt.Errorf("erro ao converter execução: %w", err)
	}
	_, err = r.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName:                 aws.String(r.table),
		Item:                      item,
		ConditionExpression:       aws.String("#status = :previous"),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":previous": &types.AttributeValueMemberS{Value: string(previous)}},
	})
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return ErrConcurrentUpdate
	}
	return err
}

func (r *ExecutionDynamoRepository) FindByStatuses(statuses ...domain.ExecutionStatus) ([]domain.Execution, error) {
	values := map[string]types.AttributeValue{}
	placeholders := ""
	for i, s := range statuses {
		key := fmt.Sprintf(":s%d", i)
		values[key] = &types.AttributeValueMemberS{Value: string(s)}
		if i > 0 {
			placeholders += ", "
		}
		placeholders += key
	}

	input := &dynamodb.ScanInput{
		TableName:                 aws.String(r.table),
		FilterExpression:          aws.String("#status IN (" + placeholders + ")"),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: values,
	}

	result := make([]domain.Execution, 0)
	paginator := dynamodb.NewScanPaginator(r.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		var batch []domain.Execution
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, fmt.Errorf("erro ao ler execuções: %w", err)
		}
		result = append(result, batch...)
	}
	return result, nil
}
