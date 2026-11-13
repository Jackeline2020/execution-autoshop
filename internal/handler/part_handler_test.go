package handler_test

import (
	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/handler"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/usecase"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupPartRouter(uc *usecase.PartUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := handler.NewPartHandler(uc)
	router.POST("/parts", h.Create)
	router.GET("/parts", h.GetAll)
	router.GET("/parts/low-stock", h.GetLowStock)
	router.GET("/parts/:id", h.GetByID)
	router.PUT("/parts/:id", h.Update)
	router.PATCH("/parts/:id/stock", h.AdjustStock)
	router.DELETE("/parts/:id", h.Delete)
	return router
}

func TestPartHandler_Create_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("Create", mock.AnythingOfType("domain.Part")).
		Return(domain.Part{ID: "p1", Name: "Filtro de óleo"}, nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	body, _ := json.Marshal(map[string]any{"name": "Filtro de óleo", "price": 30.0, "stock": 10, "min_stock": 2, "unit": "unidade"})
	req := httptest.NewRequest(http.MethodPost, "/parts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestPartHandler_Create_InvalidJSON(t *testing.T) {
	router := setupPartRouter(usecase.NewPartUseCase(&mocks.PartRepositoryMock{}))
	req := httptest.NewRequest(http.MethodPost, "/parts", bytes.NewBufferString("{invalido"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPartHandler_GetAll_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindAll").Return([]domain.Part{{ID: "p1"}}, nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/parts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPartHandler_GetLowStock_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindLowStock").Return([]domain.Part{{ID: "p1", Stock: 1, MinStock: 5}}, nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/parts/low-stock", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPartHandler_GetByID_NotFound(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindByID", "inexistente").Return(domain.Part{}, errors.New("não encontrada"))

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	req := httptest.NewRequest(http.MethodGet, "/parts/inexistente", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPartHandler_Update_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindByID", "p1").Return(domain.Part{ID: "p1"}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Part")).Return(nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	body, _ := json.Marshal(map[string]any{"name": "Filtro de óleo", "price": 35.0, "min_stock": 3, "unit": "unidade"})
	req := httptest.NewRequest(http.MethodPut, "/parts/p1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPartHandler_AdjustStock_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindByID", "p1").Return(domain.Part{ID: "p1", Stock: 10}, nil)
	repo.On("UpdateStock", "p1", 15).Return(nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	body, _ := json.Marshal(map[string]any{"quantity": 5, "reason": "entrada"})
	req := httptest.NewRequest(http.MethodPatch, "/parts/p1/stock", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPartHandler_AdjustStock_Insufficient(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindByID", "p1").Return(domain.Part{ID: "p1", Stock: 2}, nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	body, _ := json.Marshal(map[string]any{"quantity": -5, "reason": "saída"})
	req := httptest.NewRequest(http.MethodPatch, "/parts/p1/stock", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	repo.AssertNotCalled(t, "UpdateStock")
}

func TestPartHandler_Delete_Success(t *testing.T) {
	repo := &mocks.PartRepositoryMock{}
	repo.On("FindByID", "p1").Return(domain.Part{ID: "p1"}, nil)
	repo.On("Delete", "p1").Return(nil)

	router := setupPartRouter(usecase.NewPartUseCase(repo))
	req := httptest.NewRequest(http.MethodDelete, "/parts/p1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
