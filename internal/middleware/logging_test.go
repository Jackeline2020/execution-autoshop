package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"execution-autoshop/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupLoggingRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Logger())
	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, middleware.CorrelationID(c))
	})
	r.GET("/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	r.GET("/fail", func(c *gin.Context) {
		_ = c.Error(assert.AnError)
		c.Status(http.StatusInternalServerError)
	})
	return r
}

func TestLogger_KeepsIncomingCorrelationID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(middleware.CorrelationIDHeader, "corr-123")
	w := httptest.NewRecorder()
	setupLoggingRouter().ServeHTTP(w, req)

	assert.Equal(t, "corr-123", w.Header().Get(middleware.CorrelationIDHeader))
	assert.Equal(t, "corr-123", w.Body.String())
}

func TestLogger_GeneratesCorrelationID(t *testing.T) {
	w := httptest.NewRecorder()
	setupLoggingRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	id := w.Header().Get(middleware.CorrelationIDHeader)
	assert.NotEmpty(t, id)
	assert.Equal(t, id, w.Body.String())
}

func TestLogger_ErrorStatuses(t *testing.T) {
	r := setupLoggingRouter()
	for path, status := range map[string]int{"/bad": http.StatusBadRequest, "/fail": http.StatusInternalServerError} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, status, w.Code)
	}
}

func TestCorrelationID_Missing(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Empty(t, middleware.CorrelationID(c))
	c.Set("correlation_id", 42)
	assert.Empty(t, middleware.CorrelationID(c))
}

func TestLogEvent(t *testing.T) {
	middleware.LogEvent(slog.LevelInfo, "evento de teste", "k", "v")
}
