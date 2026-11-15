package mocks

import "github.com/stretchr/testify/mock"

type ProcessedMessageRepositoryMock struct {
	mock.Mock
}

func (m *ProcessedMessageRepositoryMock) Exists(messageID string) (bool, error) {
	args := m.Called(messageID)
	return args.Bool(0), args.Error(1)
}

func (m *ProcessedMessageRepositoryMock) MarkProcessed(messageID string) error {
	return m.Called(messageID).Error(0)
}
