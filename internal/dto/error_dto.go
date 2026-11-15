package dto

import "execution-autoshop/pkg/errors"

// ErrorResponse é o formato das respostas de erro da API.
type ErrorResponse struct {
	Errors []errors.ValidationError `json:"errors"`
}
