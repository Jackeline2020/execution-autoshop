package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"execution-autoshop/internal/handler"
	"execution-autoshop/internal/middleware"
	"execution-autoshop/internal/repository"
	"execution-autoshop/internal/usecase"
	"execution-autoshop/pkg/config"
	"execution-autoshop/pkg/observability"

	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent/v3/integrations/nrgin"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	nrApp := observability.NewRelicApp()

	// Em ambiente local, AWS_ENDPOINT_URL aponta para o LocalStack.
	awsCfg, err := config.LoadAWSConfig(ctx)
	if err != nil {
		log.Fatalf("erro ao carregar configuração da AWS: %v", err)
	}
	dynamo := repository.NewDynamoClient(awsCfg)

	partRepo := repository.NewPartDynamoRepository(dynamo)
	partUC := usecase.NewPartUseCase(partRepo)
	partHandler := handler.NewPartHandler(partUC)

	r := gin.New()
	r.Use(gin.Recovery())
	if nrApp != nil {
		r.Use(nrgin.Middleware(nrApp))
	}
	r.Use(middleware.Logger())

	r.GET("/health", handler.HealthCheck)

	// Rotas da oficina (funcionário / mecânico): exigem JWT
	staff := r.Group("/")
	staff.Use(middleware.AuthMiddleware())
	{
		parts := staff.Group("/parts")
		{
			parts.POST("", partHandler.Create)
			parts.GET("", partHandler.GetAll)
			parts.GET("/low-stock", partHandler.GetLowStock)
			parts.GET("/:id", partHandler.GetByID)
			parts.PUT("/:id", partHandler.Update)
			parts.PATCH("/:id/stock", partHandler.AdjustStock)
			parts.DELETE("/:id", partHandler.Delete)
		}
	}

	srv := &http.Server{
		Addr:              ":" + config.Port(),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("execution-service ouvindo na porta %s", config.Port())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("erro no servidor HTTP: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("encerrando execution-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar o servidor HTTP: %v", err)
	}
	if nrApp != nil {
		nrApp.Shutdown(5 * time.Second)
	}
}
