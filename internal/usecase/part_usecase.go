package usecase

import (
	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/repository"
	"fmt"

	"github.com/google/uuid"
)

type PartUseCase struct {
	repo repository.PartRepository
}

func NewPartUseCase(r repository.PartRepository) *PartUseCase {
	return &PartUseCase{repo: r}
}

func (u *PartUseCase) Create(name, description, unit string, price float64, stock, minStock int) (domain.Part, error) {
	part := domain.NewPart(name, description, unit, price, stock, minStock)
	part.ID = uuid.New().String()

	if err := part.Validate(); err != nil {
		return domain.Part{}, err
	}

	return u.repo.Create(part)
}

func (u *PartUseCase) GetAll() ([]domain.Part, error) {
	return u.repo.FindAll()
}

func (u *PartUseCase) GetByID(id string) (domain.Part, error) {
	return u.repo.FindByID(id)
}

func (u *PartUseCase) GetLowStock() ([]domain.Part, error) {
	return u.repo.FindLowStock()
}

func (u *PartUseCase) Update(id, name, description, unit string, price float64, minStock int) (*domain.Part, error) {
	part, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	part.Name = name
	part.Description = description
	part.Price = price
	part.MinStock = minStock
	part.Unit = unit

	if err := part.Validate(); err != nil {
		return nil, err
	}

	if err := u.repo.Update(part); err != nil {
		return nil, err
	}

	return &part, nil
}

func (u *PartUseCase) AdjustStock(id string, quantity int, reason string) (*domain.Part, error) {
	part, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	newStock := part.Stock + quantity
	if newStock < 0 {
		return nil, fmt.Errorf("estoque insuficiente. Estoque atual: %d, ajuste solicitado: %d", part.Stock, quantity)
	}

	if err := u.repo.UpdateStock(id, newStock); err != nil {
		return nil, err
	}

	part.Stock = newStock
	return &part, nil
}

func (u *PartUseCase) Delete(id string) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}
	return u.repo.Delete(id)
}
