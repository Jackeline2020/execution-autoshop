package domain

import (
	"errors"
	"fmt"
	"time"
)

type ReservationStatus string

const (
	ReservationReserved ReservationStatus = "reservada"
	ReservationReleased ReservationStatus = "liberada"
	ReservationConsumed ReservationStatus = "consumida"
)

var (
	ErrReservationNotFound = errors.New("reserva não encontrada")
	ErrReservationExists   = errors.New("já existe reserva para esta OS")
	ErrInvalidReservation  = errors.New("reserva inválida")
)

// InsufficientStockError indica que uma peça não existe ou não tem estoque
// suficiente para a reserva.
type InsufficientStockError struct {
	PartID string
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("estoque insuficiente ou peça inexistente: %s", e.PartID)
}

type ReservationItem struct {
	PartID   string `json:"part_id"  dynamodbav:"part_id"`
	Quantity int    `json:"quantity" dynamodbav:"quantity"`
}

// Reservation guarda as peças separadas para uma OS. O estoque é
// descontado no momento da reserva e devolvido se ela for liberada.
type Reservation struct {
	OrderID   string            `json:"order_id"   dynamodbav:"order_id"`
	Items     []ReservationItem `json:"items"      dynamodbav:"items"`
	Status    ReservationStatus `json:"status"     dynamodbav:"status"`
	CreatedAt string            `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt string            `json:"updated_at" dynamodbav:"updated_at"`
}

// NewReservation valida os itens e junta quantidades da mesma peça, já que
// uma transação do DynamoDB não pode alterar o mesmo item duas vezes.
func NewReservation(orderID string, items []ReservationItem) (Reservation, error) {
	if orderID == "" {
		return Reservation{}, fmt.Errorf("%w: OS é obrigatória", ErrInvalidReservation)
	}
	if len(items) == 0 {
		return Reservation{}, fmt.Errorf("%w: a reserva precisa de pelo menos uma peça", ErrInvalidReservation)
	}

	totals := make(map[string]int)
	order := make([]string, 0, len(items))
	for _, it := range items {
		if it.PartID == "" {
			return Reservation{}, fmt.Errorf("%w: peça é obrigatória", ErrInvalidReservation)
		}
		if it.Quantity <= 0 {
			return Reservation{}, fmt.Errorf("%w: quantidade da peça deve ser maior que zero", ErrInvalidReservation)
		}
		if _, seen := totals[it.PartID]; !seen {
			order = append(order, it.PartID)
		}
		totals[it.PartID] += it.Quantity
	}

	merged := make([]ReservationItem, 0, len(order))
	for _, id := range order {
		merged = append(merged, ReservationItem{PartID: id, Quantity: totals[id]})
	}

	now := time.Now().Format(time.RFC3339)
	return Reservation{
		OrderID:   orderID,
		Items:     merged,
		Status:    ReservationReserved,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
