package mocks

import (
	"context"

	"execution-autoshop/internal/messaging"

	"github.com/stretchr/testify/mock"
)

// PublisherMock registra os eventos publicados pelo tipo e pela OS.
type PublisherMock struct {
	mock.Mock
	Published []messaging.Envelope
}

func (m *PublisherMock) Publish(ctx context.Context, env messaging.Envelope) error {
	m.Published = append(m.Published, env)
	return m.Called(env.Type, env.OrderID).Error(0)
}
