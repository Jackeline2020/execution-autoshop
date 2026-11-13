package domain

import (
	"errors"
	"time"
)

type Part struct {
	ID          string  `json:"id"           dynamodbav:"part_id"`
	Name        string  `json:"name"         dynamodbav:"name"` //oleo, filtro oleo, pastiha freio, parafuso, correia dentada
	Description string  `json:"description"  dynamodbav:"description"`
	Price       float64 `json:"price"        dynamodbav:"price"`
	Stock       int     `json:"stock"        dynamodbav:"stock"`
	MinStock    int     `json:"min_stock"    dynamodbav:"min_stock"`
	Unit        string  `json:"unit"         dynamodbav:"unit"` //unidade, litro, metro, par
	CreatedAt   string  `json:"created_at"   dynamodbav:"created_at"`
}

func (p *Part) Validate() error {
	if p.Name == "" {
		return errors.New("nome da peça é obrigatório")
	}

	if p.Price <= 0 {
		return errors.New("preço da peça deve ser maior que zero")
	}

	if p.Stock < 0 {
		return errors.New("estoque não pode ser negativo")
	}

	if p.MinStock < 0 {
		return errors.New("estoque mínimo não pode ser negativo")
	}

	if p.Unit == "" {
		return errors.New("unidade de medida é obrigatória")
	}

	return nil
}

func (p *Part) IsLowStock() bool {
	return p.Stock <= p.MinStock
}

func NewPart(name, description, unit string, price float64, stock, minStock int) Part {
	return Part{
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		MinStock:    minStock,
		Unit:        unit,
		CreatedAt:   time.Now().Format(time.RFC3339),
	}
}
