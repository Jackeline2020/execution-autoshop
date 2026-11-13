package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"execution-autoshop/internal/domain"
	"execution-autoshop/internal/handler"
	"execution-autoshop/internal/mocks"
	"execution-autoshop/internal/repository"
	"execution-autoshop/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupExecutionRouter() (*gin.Engine, *mocks.ExecutionRepositoryMock, *mocks.ReservationRepositoryMock) {
	gin.SetMode(gin.TestMode)
	repo := new(mocks.ExecutionRepositoryMock)
	res := new(mocks.ReservationRepositoryMock)
	h := handler.NewExecutionHandler(usecase.NewExecutionUseCase(repo, res))

	r := gin.New()
	r.GET("/executions/queue", h.Queue)
	r.GET("/executions/:order_id", h.GetByOrderID)
	r.POST("/executions/:order_id/diagnosis", h.RegisterDiagnosis)
	r.POST("/executions/:order_id/start", h.Start)
	r.POST("/executions/:order_id/finish", h.Finish)
	return r, repo, res
}

func doRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExecutionHandler_Queue(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByStatuses", mock.Anything).Return([]domain.Execution{{OrderID: "os-1"}}, nil)

	w := doRequest(r, http.MethodGet, "/executions/queue", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "os-1")
}

func TestExecutionHandler_QueueError(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByStatuses", mock.Anything).Return([]domain.Execution(nil), errors.New("dynamo fora"))

	w := doRequest(r, http.MethodGet, "/executions/queue", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestExecutionHandler_GetByOrderID(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1"}, nil)
	repo.On("FindByOrderID", "x").Return(domain.Execution{}, domain.ErrExecutionNotFound)

	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "/executions/os-1", "").Code)
	assert.Equal(t, http.StatusNotFound, doRequest(r, http.MethodGet, "/executions/x", "").Code)
}

func TestExecutionHandler_RegisterDiagnosis(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionAwaitingDiagnosis}, nil)
	repo.On("Update", mock.Anything, domain.ExecutionAwaitingDiagnosis).Return(nil)

	w := doRequest(r, http.MethodPost, "/executions/os-1/diagnosis", `{"diagnosis":"Troca de pastilhas"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "diagnosticada")
}

func TestExecutionHandler_RegisterDiagnosisBadRequest(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionAwaitingDiagnosis}, nil)

	assert.Equal(t, http.StatusBadRequest, doRequest(r, http.MethodPost, "/executions/os-1/diagnosis", `{}`).Code)
	assert.Equal(t, http.StatusBadRequest, doRequest(r, http.MethodPost, "/executions/os-1/diagnosis", `{"diagnosis":"   "}`).Code)
}

func TestExecutionHandler_StartInvalidTransition(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionAwaitingDiagnosis}, nil)

	w := doRequest(r, http.MethodPost, "/executions/os-1/start", "")
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestExecutionHandler_StartConcurrentUpdate(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionQueued}, nil)
	repo.On("Update", mock.Anything, domain.ExecutionQueued).Return(repository.ErrConcurrentUpdate)

	assert.Equal(t, http.StatusConflict, doRequest(r, http.MethodPost, "/executions/os-1/start", "").Code)
}

func TestExecutionHandler_Finish(t *testing.T) {
	r, repo, res := setupExecutionRouter()
	repo.On("FindByOrderID", "os-1").Return(domain.Execution{OrderID: "os-1", Status: domain.ExecutionInRepair}, nil)
	repo.On("Update", mock.Anything, domain.ExecutionInRepair).Return(nil)
	res.On("Consume", "os-1").Return(nil)

	w := doRequest(r, http.MethodPost, "/executions/os-1/finish", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "finalizada")
}

func TestExecutionHandler_FinishNotFound(t *testing.T) {
	r, repo, _ := setupExecutionRouter()
	repo.On("FindByOrderID", "x").Return(domain.Execution{}, domain.ErrExecutionNotFound)

	assert.Equal(t, http.StatusNotFound, doRequest(r, http.MethodPost, "/executions/x/finish", "").Code)
}
