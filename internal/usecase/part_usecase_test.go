package usecase_test

import (
	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/usecase"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupPartUseCase() (*usecase.PartUseCase, *mocks.PartRepositoryMock) {
	repo := new(mocks.PartRepositoryMock)
	uc := usecase.NewPartUseCase(repo)
	return uc, repo
}

func TestPartUseCase_Create_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("Create", mock.AnythingOfType("domain.Part")).
		Return(domain.Part{ID: "pid", Name: "Filtro de óleo", Stock: 10}, nil)

	part, err := uc.Create("Filtro de óleo", "Filtro", "unidade", 50.0, 10, 2)

	assert.NoError(t, err)
	assert.Equal(t, "Filtro de óleo", part.Name)
	repo.AssertExpectations(t)
}

func TestPartUseCase_Create_InvalidPrice(t *testing.T) {
	uc, repo := setupPartUseCase()

	_, err := uc.Create("Filtro", "", "unidade", 0, 10, 2)

	assert.EqualError(t, err, "preço da peça deve ser maior que zero")
	repo.AssertNotCalled(t, "Create")
}

func TestPartUseCase_AdjustStock_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{ID: "pid", Name: "Filtro", Stock: 10}, nil)
	repo.On("UpdateStock", "pid", 13).Return(nil)

	part, err := uc.AdjustStock("pid", 3, "entrada")

	assert.NoError(t, err)
	assert.Equal(t, 13, part.Stock)
}

func TestPartUseCase_AdjustStock_Negative(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{ID: "pid", Name: "Filtro", Stock: 2}, nil)

	_, err := uc.AdjustStock("pid", -5, "saída")

	assert.ErrorContains(t, err, "estoque insuficiente")
	repo.AssertNotCalled(t, "UpdateStock")
}

func TestPartUseCase_GetLowStock_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindLowStock").Return([]domain.Part{
		{ID: "pid1", Name: "Filtro", Stock: 1, MinStock: 5},
	}, nil)

	parts, err := uc.GetLowStock()

	assert.NoError(t, err)
	assert.Len(t, parts, 1)
}

func TestPartUseCase_Delete_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{ID: "pid"}, nil)
	repo.On("Delete", "pid").Return(nil)

	err := uc.Delete("pid")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPartUseCase_Delete_NotFound(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{}, errors.New("peça não encontrada"))

	err := uc.Delete("pid")

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Delete")
}

func TestPartUseCase_Update_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{
		ID: "pid", Name: "Filtro", Price: 50.0, Stock: 10, MinStock: 2, Unit: "unidade",
	}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Part")).Return(nil)

	part, err := uc.Update("pid", "Filtro Novo", "Filtro atualizado", "unidade", 60.0, 3)

	assert.NoError(t, err)
	assert.Equal(t, "Filtro Novo", part.Name)
	assert.Equal(t, 60.0, part.Price)
}

func TestPartUseCase_Update_NotFound(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindByID", "pid").Return(domain.Part{}, errors.New("peça não encontrada"))

	_, err := uc.Update("pid", "Filtro", "", "unidade", 50.0, 2)

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Update")
}

func TestPartUseCase_GetAll_Success(t *testing.T) {
	uc, repo := setupPartUseCase()

	repo.On("FindAll").Return([]domain.Part{
		{ID: "pid1", Name: "Filtro"},
		{ID: "pid2", Name: "Óleo"},
	}, nil)

	parts, err := uc.GetAll()

	assert.NoError(t, err)
	assert.Len(t, parts, 2)
}
