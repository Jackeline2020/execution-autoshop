package handler

import (
	"execution-autoshop/internal/dto"
	"execution-autoshop/internal/usecase"
	pkgerrors "execution-autoshop/pkg/errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PartHandler struct {
	usecase *usecase.PartUseCase
}

func NewPartHandler(u *usecase.PartUseCase) *PartHandler {
	return &PartHandler{usecase: u}
}

// @Summary     Criar peça
// @Description Cadastra uma nova peça ou insumo com estoque inicial
// @Tags        parts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body dto.CreatePartRequest true "Dados da peça"
// @Success     201 {object} domain.Part
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /parts [post]
func (h *PartHandler) Create(c *gin.Context) {
	var req dto.CreatePartRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	part, err := h.usecase.Create(req.Name, req.Description, req.Unit, req.Price, req.Stock, req.MinStock)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "part", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusCreated, part)
}

// @Summary     Listar peças
// @Description Retorna todas as peças e insumos cadastrados
// @Tags        parts
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Part
// @Failure     401 {object} map[string]interface{}
// @Router      /parts [get]
func (h *PartHandler) GetAll(c *gin.Context) {
	parts, err := h.usecase.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, parts)
}

// @Summary     Buscar peça
// @Description Retorna uma peça pelo ID
// @Tags        parts
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID da peça"
// @Success     200 {object} domain.Part
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /parts/{id} [get]
func (h *PartHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	part, err := h.usecase.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "peça não encontrada"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, part)
}

// @Summary     Peças com estoque baixo
// @Description Retorna peças com estoque abaixo ou igual ao mínimo
// @Tags        parts
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Part
// @Failure     401 {object} map[string]interface{}
// @Router      /parts/low-stock [get]
func (h *PartHandler) GetLowStock(c *gin.Context) {
	parts, err := h.usecase.GetLowStock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, parts)
}

// @Summary     Atualizar peça
// @Description Atualiza os dados de uma peça existente (sem alterar estoque)
// @Tags        parts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                 true "ID da peça"
// @Param       request body dto.UpdatePartRequest  true "Dados atualizados"
// @Success     200 {object} domain.Part
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /parts/{id} [put]
func (h *PartHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdatePartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	part, err := h.usecase.Update(id, req.Name, req.Description, req.Unit, req.Price, req.MinStock)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "part", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, part)
}

// @Summary     Ajustar estoque
// @Description Adiciona ou remove quantidade do estoque (use negativo para saída)
// @Tags        parts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id      path string                    true "ID da peça"
// @Param       request body dto.AdjustStockRequest    true "Ajuste de estoque"
// @Success     200 {object} domain.Part
// @Failure     400 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     422 {object} map[string]interface{}
// @Router      /parts/{id}/stock [patch]
func (h *PartHandler) AdjustStock(c *gin.Context) {
	id := c.Param("id")

	var req dto.AdjustStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"errors": pkgerrors.ParseValidationErrors(err),
		})
		return
	}

	part, err := h.usecase.AdjustStock(id, req.Quantity, req.Reason)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "stock", Message: err.Error()},
			},
		})
		return
	}

	c.JSON(http.StatusOK, part)
}

// @Summary     Deletar peça
// @Description Remove uma peça do sistema
// @Tags        parts
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "ID da peça"
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} map[string]interface{}
// @Failure     404 {object} map[string]interface{}
// @Router      /parts/{id} [delete]
func (h *PartHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.usecase.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"errors": []pkgerrors.ValidationError{
				{Field: "id", Message: "peça não encontrada"},
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "peça deletada com sucesso"})
}
