package handler

import (
	"errors"
	"net/http"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/dto"
	"execution-autoshop/internal/repository"
	"execution-autoshop/internal/usecase"
	pkgerrors "execution-autoshop/pkg/errors"

	"github.com/gin-gonic/gin"
)

type ExecutionHandler struct {
	usecase *usecase.ExecutionUseCase
}

func NewExecutionHandler(u *usecase.ExecutionUseCase) *ExecutionHandler {
	return &ExecutionHandler{usecase: u}
}

// @Summary     Fila da oficina
// @Description Lista as OS que aguardam diagnóstico ou reparo, das mais antigas para as mais recentes
// @Tags        executions
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} domain.Execution
// @Router      /executions/queue [get]
func (h *ExecutionHandler) Queue(c *gin.Context) {
	items, err := h.usecase.Queue()
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

// @Summary     Consultar execução
// @Description Mostra o andamento do diagnóstico e do reparo de uma OS
// @Tags        executions
// @Produce     json
// @Security    BearerAuth
// @Param       order_id path string true "ID da OS"
// @Success     200 {object} domain.Execution
// @Failure     404 {object} map[string][]pkgerrors.ValidationError
// @Router      /executions/{order_id} [get]
func (h *ExecutionHandler) GetByOrderID(c *gin.Context) {
	e, err := h.usecase.GetByOrderID(c.Param("order_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

// @Summary     Registrar diagnóstico
// @Tags        executions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       order_id path string true "ID da OS"
// @Param       request body dto.DiagnosisRequest true "Diagnóstico"
// @Success     200 {object} domain.Execution
// @Failure     400 {object} map[string][]pkgerrors.ValidationError
// @Failure     404 {object} map[string][]pkgerrors.ValidationError
// @Failure     409 {object} map[string][]pkgerrors.ValidationError
// @Router      /executions/{order_id}/diagnosis [post]
func (h *ExecutionHandler) RegisterDiagnosis(c *gin.Context) {
	var req dto.DiagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": pkgerrors.ParseValidationErrors(err)})
		return
	}
	e, err := h.usecase.RegisterDiagnosis(c.Param("order_id"), req.Diagnosis)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

// @Summary     Iniciar reparo
// @Tags        executions
// @Produce     json
// @Security    BearerAuth
// @Param       order_id path string true "ID da OS"
// @Success     200 {object} domain.Execution
// @Failure     404 {object} map[string][]pkgerrors.ValidationError
// @Failure     409 {object} map[string][]pkgerrors.ValidationError
// @Router      /executions/{order_id}/start [post]
func (h *ExecutionHandler) Start(c *gin.Context) {
	e, err := h.usecase.Start(c.Param("order_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

// @Summary     Finalizar reparo
// @Tags        executions
// @Produce     json
// @Security    BearerAuth
// @Param       order_id path string true "ID da OS"
// @Success     200 {object} domain.Execution
// @Failure     404 {object} map[string][]pkgerrors.ValidationError
// @Failure     409 {object} map[string][]pkgerrors.ValidationError
// @Router      /executions/{order_id}/finish [post]
func (h *ExecutionHandler) Finish(c *gin.Context) {
	e, err := h.usecase.Finish(c.Param("order_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, e)
}

// respondError traduz erros de domínio para o status HTTP adequado.
func respondError(c *gin.Context, err error) {
	var transErr *domain.InvalidTransitionError
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, domain.ErrExecutionNotFound):
		status = http.StatusNotFound
	case errors.As(err, &transErr), errors.Is(err, repository.ErrConcurrentUpdate):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrDiagnosisRequired), errors.Is(err, domain.ErrOrderRequired):
		status = http.StatusBadRequest
	}

	c.JSON(status, gin.H{"errors": []pkgerrors.ValidationError{{Field: "execution", Message: err.Error()}}})
}
