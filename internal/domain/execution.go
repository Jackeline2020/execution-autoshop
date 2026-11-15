package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type ExecutionStatus string

const (
	ExecutionAwaitingDiagnosis ExecutionStatus = "aguardando_diagnostico"
	ExecutionDiagnosed         ExecutionStatus = "diagnosticada"
	ExecutionQueued            ExecutionStatus = "na_fila"
	ExecutionInRepair          ExecutionStatus = "em_reparo"
	ExecutionFinished          ExecutionStatus = "finalizada"
)

var (
	ErrExecutionNotFound = errors.New("execução não encontrada")
	ErrExecutionExists   = errors.New("já existe execução para esta OS")
	ErrOrderRequired     = errors.New("OS é obrigatória")
	ErrDiagnosisRequired = errors.New("diagnóstico é obrigatório")
)

// InvalidTransitionError indica uma mudança de status fora da ordem do
// fluxo (ex.: finalizar um reparo que não foi iniciado).
type InvalidTransitionError struct {
	From, To ExecutionStatus
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("transição de status inválida: %s → %s", e.From, e.To)
}

// Execution acompanha o trabalho da oficina sobre uma OS, do diagnóstico
// ao fim do reparo. Cada mudança de status registra a data e hora em que
// aconteceu.
type Execution struct {
	OrderID string `json:"order_id"               dynamodbav:"order_id"`
	SagaID  string `json:"saga_id,omitempty"      dynamodbav:"saga_id,omitempty"`
	// CorrelationID liga os eventos desta OS ao mesmo rastro no New Relic.
	CorrelationID string          `json:"correlation_id,omitempty" dynamodbav:"correlation_id,omitempty"`
	Status        ExecutionStatus `json:"status"                 dynamodbav:"status"`
	Diagnosis     string          `json:"diagnosis,omitempty"    dynamodbav:"diagnosis,omitempty"`
	CreatedAt     string          `json:"created_at"             dynamodbav:"created_at"`
	DiagnosedAt   string          `json:"diagnosed_at,omitempty" dynamodbav:"diagnosed_at,omitempty"`
	QueuedAt      string          `json:"queued_at,omitempty"    dynamodbav:"queued_at,omitempty"`
	StartedAt     string          `json:"started_at,omitempty"   dynamodbav:"started_at,omitempty"`
	FinishedAt    string          `json:"finished_at,omitempty"  dynamodbav:"finished_at,omitempty"`
	UpdatedAt     string          `json:"updated_at"             dynamodbav:"updated_at"`
}

var executionTransitions = map[ExecutionStatus]ExecutionStatus{
	ExecutionAwaitingDiagnosis: ExecutionDiagnosed,
	ExecutionDiagnosed:         ExecutionQueued,
	ExecutionQueued:            ExecutionInRepair,
	ExecutionInRepair:          ExecutionFinished,
}

func NewExecution(orderID string) (Execution, error) {
	if orderID == "" {
		return Execution{}, ErrOrderRequired
	}
	now := time.Now().Format(time.RFC3339)
	return Execution{
		OrderID:   orderID,
		Status:    ExecutionAwaitingDiagnosis,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (e *Execution) RegisterDiagnosis(diagnosis string) error {
	diagnosis = strings.TrimSpace(diagnosis)
	if diagnosis == "" {
		return ErrDiagnosisRequired
	}
	if err := e.transitionTo(ExecutionDiagnosed); err != nil {
		return err
	}
	e.Diagnosis = diagnosis
	return nil
}

func (e *Execution) Enqueue() error { return e.transitionTo(ExecutionQueued) }
func (e *Execution) Start() error   { return e.transitionTo(ExecutionInRepair) }
func (e *Execution) Finish() error  { return e.transitionTo(ExecutionFinished) }

// WaitingSince devolve desde quando a execução espera pelo próximo passo
// da oficina, usado para ordenar a fila (mais antigas primeiro).
func (e *Execution) WaitingSince() string {
	if e.Status == ExecutionQueued {
		return e.QueuedAt
	}
	return e.CreatedAt
}

func (e *Execution) transitionTo(next ExecutionStatus) error {
	if executionTransitions[e.Status] != next {
		return &InvalidTransitionError{From: e.Status, To: next}
	}

	now := time.Now().Format(time.RFC3339)
	switch next {
	case ExecutionDiagnosed:
		e.DiagnosedAt = now
	case ExecutionQueued:
		e.QueuedAt = now
	case ExecutionInRepair:
		e.StartedAt = now
	case ExecutionFinished:
		e.FinishedAt = now
	}
	e.Status = next
	e.UpdatedAt = now
	return nil
}
