package middleware

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// CorrelationIDHeader é o header usado pra correlacionar requisições entre
// serviços (lambda-auth-autoshop → app-autoshop, e chamadas encadeadas
// dentro do próprio app) — atende ao requisito de "logs estruturados (JSON),
// incluindo correlação entre requisições" da Fase 3.
const CorrelationIDHeader = "X-Correlation-ID"

// structuredLogger é compartilhado por toda a aplicação — slog com handler
// JSON escreve uma linha por evento, sem timestamps/formatação ad-hoc
// espalhados pelo código, e é o formato que ferramentas de observabilidade
// (New Relic, CloudWatch, etc.) esperam pra fazer parsing automático.
var structuredLogger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// Logger é um substituto estruturado do logger padrão do Gin
// (gin.Default() usa um logger em texto simples, não parseável). Cada
// requisição gera exatamente uma linha de log JSON com: correlation_id
// (aceita o do header, ou gera um novo — e sempre devolve no response pra
// quem chamou conseguir rastrear do lado dele também), método, rota,
// status, latência e, quando o New Relic está habilitado, o trace_id/
// span_id da transação — permitindo ir do log direto pro trace no New
// Relic sem precisar cruzar timestamps manualmente.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		c.Set("correlation_id", correlationID)
		c.Header(CorrelationIDHeader, correlationID)

		c.Next()

		attrs := []any{
			"correlation_id", correlationID,
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		}

		if txn := newrelic.FromContext(c.Request.Context()); txn != nil {
			meta := txn.GetLinkingMetadata()
			if meta.TraceID != "" {
				attrs = append(attrs, "trace_id", meta.TraceID, "span_id", meta.SpanID)
			}
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		} else if c.Writer.Status() >= 400 {
			level = slog.LevelWarn
		}

		structuredLogger.Log(c.Request.Context(), level, "http_request", attrs...)
	}
}

// CorrelationID devolve o correlation_id da requisição atual — usado pra
// incluir o mesmo ID em eventos customizados de negócio (ver
// internal/usecase/order_usecase.go), garantindo que um log de "OS mudou de
// status" e o log HTTP que o originou tenham o mesmo correlation_id.
func CorrelationID(c *gin.Context) string {
	if v, exists := c.Get("correlation_id"); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// LogEvent registra um evento de negócio estruturado (fora do ciclo de
// requisição HTTP, ou complementar a ele) — usado por exemplo pra falhas no
// processamento de ordens de serviço, que é um dos itens monitorados
// explicitamente pelo requisito da Fase 3.
func LogEvent(level slog.Level, msg string, attrs ...any) {
	structuredLogger.Log(context.Background(), level, msg, attrs...)
}
