package domain_test

import (
	"testing"

	"execution-autoshop/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReservation_MergesSamePart(t *testing.T) {
	res, err := domain.NewReservation("os-1", []domain.ReservationItem{
		{PartID: "p1", Quantity: 1},
		{PartID: "p2", Quantity: 3},
		{PartID: "p1", Quantity: 2},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationReserved, res.Status)
	assert.Equal(t, []domain.ReservationItem{
		{PartID: "p1", Quantity: 3},
		{PartID: "p2", Quantity: 3},
	}, res.Items)
	assert.NotEmpty(t, res.CreatedAt)
}

func TestNewReservation_Invalid(t *testing.T) {
	cases := map[string]struct {
		orderID string
		items   []domain.ReservationItem
	}{
		"sem OS":              {"", []domain.ReservationItem{{PartID: "p1", Quantity: 1}}},
		"sem itens":           {"os-1", nil},
		"sem peça":            {"os-1", []domain.ReservationItem{{Quantity: 1}}},
		"quantidade zero":     {"os-1", []domain.ReservationItem{{PartID: "p1", Quantity: 0}}},
		"quantidade negativa": {"os-1", []domain.ReservationItem{{PartID: "p1", Quantity: -2}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewReservation(tc.orderID, tc.items)
			assert.Error(t, err)
		})
	}
}

func TestInsufficientStockError(t *testing.T) {
	err := &domain.InsufficientStockError{PartID: "p9"}
	assert.Contains(t, err.Error(), "p9")
}

func TestNewReservation_InvalidIsTyped(t *testing.T) {
	_, err := domain.NewReservation("os-1", nil)
	assert.ErrorIs(t, err, domain.ErrInvalidReservation)
}
