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

	"execution-autoshop/internal/consumer"
	"execution-autoshop/internal/handler"
	"execution-autoshop/internal/messaging"
	"execution-autoshop/internal/middleware"
	"execution-autoshop/internal/repository"
	"execution-autoshop/internal/usecase"
	"execution-autoshop/pkg/config"
	"execution-autoshop/pkg/observability"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
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

	// Filas da Saga: comandos chegam em execution-commands, eventos saem em
	// os-events (ADR-006).
	sqsClient := sqs.NewFromConfig(awsCfg)
	commandsURL, err := messaging.QueueURL(ctx, sqsClient, config.Env("EXECUTION_COMMANDS_QUEUE", "execution-commands"))
	if err != nil {
		log.Fatal(err)
	}
	eventsURL, err := messaging.QueueURL(ctx, sqsClient, config.Env("OS_EVENTS_QUEUE", "os-events"))
	if err != nil {
		log.Fatal(err)
	}
	publisher := messaging.NewSQSPublisher(sqsClient, eventsURL)

	reservationRepo := repository.NewReservationDynamoRepository(dynamo)
	reservationUC := usecase.NewReservationUseCase(reservationRepo)
	executionRepo := repository.NewExecutionDynamoRepository(dynamo)
	executionUC := usecase.NewExecutionUseCase(executionRepo, reservationRepo, publisher)
	executionHandler := handler.NewExecutionHandler(executionUC)

	commandHandler := consumer.NewCommandHandler(executionUC, reservationUC, publisher)
	processedRepo := repository.NewProcessedMessageDynamoRepository(dynamo)
	commandConsumer := consumer.NewSQSConsumer(sqsClient, commandsURL, commandHandler, processedRepo)
	consumerDone := make(chan struct{})
	go func() {
		commandConsumer.Run(ctx)
		close(consumerDone)
	}()

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

		executions := staff.Group("/executions")
		{
			executions.GET("/queue", executionHandler.Queue)
			executions.GET("/:order_id", executionHandler.GetByOrderID)
			executions.POST("/:order_id/diagnosis", executionHandler.RegisterDiagnosis)
			executions.POST("/:order_id/start", executionHandler.Start)
			executions.POST("/:order_id/finish", executionHandler.Finish)
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
	<-consumerDone
	if nrApp != nil {
		nrApp.Shutdown(5 * time.Second)
	}
}
