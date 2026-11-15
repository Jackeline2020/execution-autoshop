package dto

type CreatePartRequest struct {
	Name        string  `json:"name"        binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price"       binding:"required"`
	Stock       int     `json:"stock"`
	MinStock    int     `json:"min_stock"`
	Unit        string  `json:"unit"        binding:"required"`
}

type UpdatePartRequest struct {
	Name        string  `json:"name"        binding:"required"`
	Description string  `json:"description"`
	Price       float64 `json:"price"       binding:"required"`
	MinStock    int     `json:"min_stock"`
	Unit        string  `json:"unit"        binding:"required"`
}

type AdjustStockRequest struct {
	Quantity int    `json:"quantity" binding:"required"`
	Reason   string `json:"reason"   binding:"required"` //entrada, saída, ajuste
}
