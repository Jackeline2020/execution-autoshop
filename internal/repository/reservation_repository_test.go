package repository_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/repository"
	"execution-autoshop/pkg/config"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReservationRepos(t *testing.T) (*repository.ReservationDynamoRepository, *repository.PartDynamoRepository) {
	t.Helper()
	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL não definida: teste de integração ignorado")
	}
	cfg, err := config.LoadAWSConfig(context.Background())
	require.NoError(t, err)
	client := repository.NewDynamoClient(cfg)
	return repository.NewReservationDynamoRepository(client), repository.NewPartDynamoRepository(client)
}

func createPart(t *testing.T, parts *repository.PartDynamoRepository, stock int) domain.Part {
	t.Helper()
	p := newTestPart(stock, 0)
	_, err := parts.Create(p)
	require.NoError(t, err)
	t.Cleanup(func() { _ = parts.Delete(p.ID) })
	return p
}

func stockOf(t *testing.T, parts *repository.PartDynamoRepository, id string) int {
	t.Helper()
	p, err := parts.FindByID(id)
	require.NoError(t, err)
	return p.Stock
}

func reservation(t *testing.T, items ...domain.ReservationItem) domain.Reservation {
	t.Helper()
	res, err := domain.NewReservation(uuid.NewString(), items)
	require.NoError(t, err)
	return res
}

func TestReservation_ReserveDiscountsStock(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	oil := createPart(t, parts, 10)
	filter := createPart(t, parts, 5)

	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 4}, domain.ReservationItem{PartID: filter.ID, Quantity: 1})
	require.NoError(t, resRepo.Reserve(res))

	assert.Equal(t, 6, stockOf(t, parts, oil.ID))
	assert.Equal(t, 4, stockOf(t, parts, filter.ID))

	found, err := resRepo.FindByOrderID(res.OrderID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationReserved, found.Status)
	assert.Equal(t, res.Items, found.Items)
}

func TestReservation_InsufficientStockReservesNothing(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	oil := createPart(t, parts, 10)
	filter := createPart(t, parts, 1)

	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 2}, domain.ReservationItem{PartID: filter.ID, Quantity: 3})
	err := resRepo.Reserve(res)

	var stockErr *domain.InsufficientStockError
	require.ErrorAs(t, err, &stockErr)
	assert.Equal(t, filter.ID, stockErr.PartID)
	// Nada foi descontado: nem a peça que tinha estoque
	assert.Equal(t, 10, stockOf(t, parts, oil.ID))
	assert.Equal(t, 1, stockOf(t, parts, filter.ID))
	_, err = resRepo.FindByOrderID(res.OrderID)
	assert.ErrorIs(t, err, domain.ErrReservationNotFound)
}

func TestReservation_UnknownPart(t *testing.T) {
	resRepo, _ := newReservationRepos(t)
	res := reservation(t, domain.ReservationItem{PartID: uuid.NewString(), Quantity: 1})

	var stockErr *domain.InsufficientStockError
	assert.ErrorAs(t, resRepo.Reserve(res), &stockErr)
}

func TestReservation_DuplicateDoesNotDiscountTwice(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	oil := createPart(t, parts, 10)
	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 3})

	require.NoError(t, resRepo.Reserve(res))
	assert.ErrorIs(t, resRepo.Reserve(res), domain.ErrReservationExists)
	assert.Equal(t, 7, stockOf(t, parts, oil.ID))
}

func TestReservation_ReleaseIsIdempotent(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	oil := createPart(t, parts, 10)
	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 3})
	require.NoError(t, resRepo.Reserve(res))

	require.NoError(t, resRepo.Release(res.OrderID))
	require.NoError(t, resRepo.Release(res.OrderID)) // segunda vez não devolve de novo

	assert.Equal(t, 10, stockOf(t, parts, oil.ID))
	found, err := resRepo.FindByOrderID(res.OrderID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationReleased, found.Status)
}

func TestReservation_ReleaseWithoutReservation(t *testing.T) {
	resRepo, _ := newReservationRepos(t)
	assert.NoError(t, resRepo.Release(uuid.NewString()))
}

func TestReservation_Consume(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	oil := createPart(t, parts, 10)
	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 2})
	require.NoError(t, resRepo.Reserve(res))

	require.NoError(t, resRepo.Consume(res.OrderID))
	require.NoError(t, resRepo.Consume(res.OrderID)) // idempotente

	assert.Equal(t, 8, stockOf(t, parts, oil.ID))
	// Reserva consumida não é mais liberada
	require.NoError(t, resRepo.Release(res.OrderID))
	assert.Equal(t, 8, stockOf(t, parts, oil.ID))
}

func TestReservation_ConsumeErrors(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	assert.ErrorIs(t, resRepo.Consume(uuid.NewString()), domain.ErrReservationNotFound)

	oil := createPart(t, parts, 10)
	res := reservation(t, domain.ReservationItem{PartID: oil.ID, Quantity: 1})
	require.NoError(t, resRepo.Reserve(res))
	require.NoError(t, resRepo.Release(res.OrderID))
	assert.Error(t, resRepo.Consume(res.OrderID))
}

// Duas OS disputam a última unidade da peça ao mesmo tempo: só uma pode
// conseguir, e o estoque nunca fica negativo.
func TestReservation_ConcurrentLastUnit(t *testing.T) {
	resRepo, parts := newReservationRepos(t)
	part := createPart(t, parts, 1)

	const competitors = 5
	var wg sync.WaitGroup
	results := make(chan error, competitors)
	for i := 0; i < competitors; i++ {
		res := reservation(t, domain.ReservationItem{PartID: part.ID, Quantity: 1})
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- resRepo.Reserve(res)
		}()
	}
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	assert.Equal(t, 1, succeeded)
	assert.Equal(t, 0, stockOf(t, parts, part.ID))
}
