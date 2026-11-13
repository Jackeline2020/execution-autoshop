package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck é uma rota pública e sem integração com o banco usada por Docker HEALTHCHECK e Kubernetes liveness/readiness probes

// @Summary     Health check
// @Description Indica se o processo da API está de pé
// @Tags        health
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /health [get]
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
