package repository

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"execution-autoshop/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ErrPartNotFound é devolvido quando a peça não existe na tabela.
var ErrPartNotFound = errors.New("peça não encontrada")

type PartRepository interface {
	Create(part domain.Part) (domain.Part, error)
	FindAll() ([]domain.Part, error)
	FindByID(id string) (domain.Part, error)
	FindLowStock() ([]domain.Part, error)
	Update(part domain.Part) error
	UpdateStock(id string, quantity int) error
	Delete(id string) error
}

type PartDynamoRepository struct {
	client *dynamodb.Client
	table  string
}

func NewPartDynamoRepository(client *dynamodb.Client) *PartDynamoRepository {
	return &PartDynamoRepository{client: client, table: tableName("PARTS_TABLE", "parts")}
}

func (r *PartDynamoRepository) key(id string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"part_id": &types.AttributeValueMemberS{Value: id}}
}

func (r *PartDynamoRepository) Create(part domain.Part) (domain.Part, error) {
	item, err := attributevalue.MarshalMap(part)
	if err != nil {
		return part, fmt.Errorf("erro ao converter peça: %w", err)
	}

	_, err = r.client.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName:           aws.String(r.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(part_id)"),
	})
	if err != nil {
		return part, fmt.Errorf("erro ao criar peça: %w", err)
	}
	return part, nil
}

func (r *PartDynamoRepository) FindAll() ([]domain.Part, error) {
	return r.scan(nil)
}

func (r *PartDynamoRepository) FindByID(id string) (domain.Part, error) {
	out, err := r.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            r.key(id),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return domain.Part{}, err
	}
	if out.Item == nil {
		return domain.Part{}, ErrPartNotFound
	}

	var p domain.Part
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return domain.Part{}, fmt.Errorf("erro ao ler peça: %w", err)
	}
	return p, nil
}

// FindLowStock usa um filtro que compara dois atributos do mesmo item
// (estoque <= estoque mínimo).
func (r *PartDynamoRepository) FindLowStock() ([]domain.Part, error) {
	return r.scan(&dynamodb.ScanInput{
		FilterExpression:         aws.String("#stock <= #min_stock"),
		ExpressionAttributeNames: map[string]string{"#stock": "stock", "#min_stock": "min_stock"},
	})
}

func (r *PartDynamoRepository) Update(part domain.Part) error {
	_, err := r.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.table),
		Key:                 r.key(part.ID),
		ConditionExpression: aws.String("attribute_exists(part_id)"),
		// Todos os atributos usam apelido (#...): vários nomes comuns, como
		// "name" e "unit", são palavras reservadas no DynamoDB.
		UpdateExpression: aws.String("SET #name = :name, #description = :description, #price = :price, #min_stock = :min_stock, #unit = :unit"),
		ExpressionAttributeNames: map[string]string{
			"#name":        "name",
			"#description": "description",
			"#price":       "price",
			"#min_stock":   "min_stock",
			"#unit":        "unit",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":        &types.AttributeValueMemberS{Value: part.Name},
			":description": &types.AttributeValueMemberS{Value: part.Description},
			":price":       &types.AttributeValueMemberN{Value: strconv.FormatFloat(part.Price, 'f', -1, 64)},
			":min_stock":   &types.AttributeValueMemberN{Value: strconv.Itoa(part.MinStock)},
			":unit":        &types.AttributeValueMemberS{Value: part.Unit},
		},
	})
	return translateConditionErr(err)
}

func (r *PartDynamoRepository) UpdateStock(id string, quantity int) error {
	_, err := r.client.UpdateItem(context.Background(), &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.table),
		Key:                 r.key(id),
		ConditionExpression: aws.String("attribute_exists(part_id)"),
		UpdateExpression:    aws.String("SET #stock = :stock"),
		ExpressionAttributeNames: map[string]string{
			"#stock": "stock",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":stock": &types.AttributeValueMemberN{Value: strconv.Itoa(quantity)},
		},
	})
	return translateConditionErr(err)
}

func (r *PartDynamoRepository) Delete(id string) error {
	_, err := r.client.DeleteItem(context.Background(), &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key:       r.key(id),
	})
	return err
}

// scan percorre todas as páginas do resultado e devolve as peças
// ordenadas por nome, como na versão relacional.
func (r *PartDynamoRepository) scan(input *dynamodb.ScanInput) ([]domain.Part, error) {
	if input == nil {
		input = &dynamodb.ScanInput{}
	}
	input.TableName = aws.String(r.table)

	parts := make([]domain.Part, 0)
	paginator := dynamodb.NewScanPaginator(r.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		var batch []domain.Part
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &batch); err != nil {
			return nil, fmt.Errorf("erro ao ler peças: %w", err)
		}
		parts = append(parts, batch...)
	}

	sort.Slice(parts, func(i, j int) bool { return parts[i].Name < parts[j].Name })
	return parts, nil
}

func translateConditionErr(err error) error {
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return ErrPartNotFound
	}
	return err
}
