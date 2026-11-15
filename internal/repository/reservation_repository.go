package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"execution-autoshop/internal/domain"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type ReservationRepository interface {
	Reserve(res domain.Reservation) error
	Release(orderID string) error
	Consume(orderID string) error
	FindByOrderID(orderID string) (domain.Reservation, error)
}

type ReservationDynamoRepository struct {
	client     *dynamodb.Client
	table      string
	partsTable string
}

func NewReservationDynamoRepository(client *dynamodb.Client) *ReservationDynamoRepository {
	return &ReservationDynamoRepository{
		client:     client,
		table:      tableName("RESERVATIONS_TABLE", "reservations"),
		partsTable: tableName("PARTS_TABLE", "parts"),
	}
}

// Reserve desconta o estoque de todas as peças e grava a reserva em uma
// única transação: ou tudo é gravado, ou nada. Cada peça só é descontada
// se tiver estoque suficiente naquele instante, o que impede que duas OS
// reservem a mesma unidade.
func (r *ReservationDynamoRepository) Reserve(res domain.Reservation) error {
	item, err := attributevalue.MarshalMap(res)
	if err != nil {
		return fmt.Errorf("erro ao converter reserva: %w", err)
	}

	// A gravação da reserva é a primeira operação; as peças vêm em seguida,
	// na mesma ordem de res.Items (usado para identificar qual falhou).
	ops := []types.TransactWriteItem{{
		Put: &types.Put{
			TableName:           aws.String(r.table),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(order_id)"),
		},
	}}
	for _, it := range res.Items {
		ops = append(ops, r.stockUpdate(it, "-", "attribute_exists(part_id) AND #stock >= :qty"))
	}

	_, err = r.client.TransactWriteItems(context.Background(), &dynamodb.TransactWriteItemsInput{TransactItems: ops})
	if failed, ok := failedIndex(err); ok {
		if failed == 0 {
			return domain.ErrReservationExists
		}
		return &domain.InsufficientStockError{PartID: res.Items[failed-1].PartID}
	}
	return err
}

// Release devolve o estoque e marca a reserva como liberada. Só age sobre
// reservas ainda ativas: chamar de novo não devolve o estoque em dobro.
func (r *ReservationDynamoRepository) Release(orderID string) error {
	res, err := r.FindByOrderID(orderID)
	if errors.Is(err, domain.ErrReservationNotFound) {
		return nil // nada foi reservado, nada a devolver
	}
	if err != nil {
		return err
	}
	if res.Status != domain.ReservationReserved {
		return nil
	}

	ops := []types.TransactWriteItem{r.statusUpdate(orderID, domain.ReservationReserved, domain.ReservationReleased)}
	for _, it := range res.Items {
		ops = append(ops, r.stockUpdate(it, "+", "attribute_exists(part_id)"))
	}

	_, err = r.client.TransactWriteItems(context.Background(), &dynamodb.TransactWriteItemsInput{TransactItems: ops})
	if failed, ok := failedIndex(err); ok && failed == 0 {
		return nil // outra execução já liberou ou consumiu a reserva
	}
	return err
}

// Consume marca a reserva como usada no reparo. O estoque já foi
// descontado na reserva, então nada muda nas peças.
func (r *ReservationDynamoRepository) Consume(orderID string) error {
	res, err := r.FindByOrderID(orderID)
	if err != nil {
		return err
	}
	switch res.Status {
	case domain.ReservationConsumed:
		return nil
	case domain.ReservationReleased:
		return errors.New("reserva já foi liberada e não pode ser consumida")
	}

	_, err = r.client.TransactWriteItems(context.Background(), &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{r.statusUpdate(orderID, domain.ReservationReserved, domain.ReservationConsumed)},
	})
	return err
}

func (r *ReservationDynamoRepository) FindByOrderID(orderID string) (domain.Reservation, error) {
	out, err := r.client.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            map[string]types.AttributeValue{"order_id": &types.AttributeValueMemberS{Value: orderID}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return domain.Reservation{}, err
	}
	if out.Item == nil {
		return domain.Reservation{}, domain.ErrReservationNotFound
	}

	var res domain.Reservation
	if err := attributevalue.UnmarshalMap(out.Item, &res); err != nil {
		return domain.Reservation{}, fmt.Errorf("erro ao ler reserva: %w", err)
	}
	return res, nil
}

func (r *ReservationDynamoRepository) stockUpdate(it domain.ReservationItem, op, condition string) types.TransactWriteItem {
	return types.TransactWriteItem{
		Update: &types.Update{
			TableName:                aws.String(r.partsTable),
			Key:                      map[string]types.AttributeValue{"part_id": &types.AttributeValueMemberS{Value: it.PartID}},
			UpdateExpression:         aws.String("SET #stock = #stock " + op + " :qty"),
			ConditionExpression:      aws.String(condition),
			ExpressionAttributeNames: map[string]string{"#stock": "stock"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":qty": &types.AttributeValueMemberN{Value: strconv.Itoa(it.Quantity)},
			},
		},
	}
}

func (r *ReservationDynamoRepository) statusUpdate(orderID string, from, to domain.ReservationStatus) types.TransactWriteItem {
	return types.TransactWriteItem{
		Update: &types.Update{
			TableName:                aws.String(r.table),
			Key:                      map[string]types.AttributeValue{"order_id": &types.AttributeValueMemberS{Value: orderID}},
			UpdateExpression:         aws.String("SET #status = :to, #updated_at = :now"),
			ConditionExpression:      aws.String("#status = :from"),
			ExpressionAttributeNames: map[string]string{"#status": "status", "#updated_at": "updated_at"},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":from": &types.AttributeValueMemberS{Value: string(from)},
				":to":   &types.AttributeValueMemberS{Value: string(to)},
				":now":  &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			},
		},
	}
}

// failedIndex devolve a posição da operação cuja condição falhou quando uma
// transação é cancelada.
func failedIndex(err error) (int, bool) {
	var canceled *types.TransactionCanceledException
	if !errors.As(err, &canceled) {
		return 0, false
	}
	for i, reason := range canceled.CancellationReasons {
		if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
			return i, true
		}
	}
	return 0, false
}
