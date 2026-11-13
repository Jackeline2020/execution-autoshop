package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func ParseValidationErrors(err error) []ValidationError {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return []ValidationError{{Field: "unknown", Message: err.Error()}}
	}

	var result []ValidationError
	for _, fe := range ve {
		result = append(result, ValidationError{
			Field:   formatField(fe.Namespace()),
			Message: formatMessage(fe),
		})
	}
	return result
}

func formatField(namespace string) string {
	// Remove o prefixo da struct (ex: "CreateCustomerRequest.")
	parts := strings.SplitN(namespace, ".", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[1])
	}
	return strings.ToLower(namespace)
}

func formatMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s é obrigatório", formatFieldName(fe.Field()))
	case "email":
		return "formato de email inválido"
	case "min":
		return fmt.Sprintf("%s deve ter no mínimo %s caracteres", formatFieldName(fe.Field()), fe.Param())
	case "max":
		return fmt.Sprintf("%s deve ter no máximo %s caracteres", formatFieldName(fe.Field()), fe.Param())
	default:
		return fmt.Sprintf("%s é inválido", formatFieldName(fe.Field()))
	}
}

func formatFieldName(field string) string {
	switch field {
	case "Name":
		return "nome"
	case "CPF":
		return "CPF"
	case "CNPJ":
		return "CNPJ"
	case "Email":
		return "email"
	case "Phone":
		return "telefone"
	case "Street":
		return "rua"
	case "Number":
		return "número"
	case "City":
		return "cidade"
	case "State":
		return "estado"
	case "ZipCode":
		return "CEP"
	default:
		return field
	}
}
