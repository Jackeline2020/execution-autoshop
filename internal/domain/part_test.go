package domain_test

import (
	"execution-autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartValidate_Success(t *testing.T) {
	part := domain.NewPart("Filtro de óleo", "Filtro", "unidade", 50.0, 10, 2)
	part.ID = "1"

	err := part.Validate()
	assert.NoError(t, err)
}

func TestPartValidate_MissingName(t *testing.T) {
	part := domain.NewPart("", "Filtro", "unidade", 50.0, 10, 2)

	err := part.Validate()
	assert.EqualError(t, err, "nome da peça é obrigatório")
}

func TestPartValidate_InvalidPrice(t *testing.T) {
	part := domain.NewPart("Filtro", "Filtro", "unidade", 0, 10, 2)

	err := part.Validate()
	assert.EqualError(t, err, "preço da peça deve ser maior que zero")
}

func TestPartValidate_NegativeStock(t *testing.T) {
	part := domain.NewPart("Filtro", "Filtro", "unidade", 50.0, -1, 2)

	err := part.Validate()
	assert.EqualError(t, err, "estoque não pode ser negativo")
}

func TestPartValidate_MissingUnit(t *testing.T) {
	part := domain.NewPart("Filtro", "Filtro", "", 50.0, 10, 2)

	err := part.Validate()
	assert.EqualError(t, err, "unidade de medida é obrigatória")
}

func TestPartIsLowStock_True(t *testing.T) {
	part := domain.NewPart("Filtro", "", "unidade", 50.0, 2, 5)

	assert.True(t, part.IsLowStock())
}

func TestPartIsLowStock_False(t *testing.T) {
	part := domain.NewPart("Filtro", "", "unidade", 50.0, 10, 5)

	assert.False(t, part.IsLowStock())
}
