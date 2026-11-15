package errors_test

import (
	"errors"
	pkgerrors "execution-autoshop/pkg/errors"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

type sampleRequest struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Code  string `validate:"min=3,max=5"`
}

func TestParseValidationErrors_RequiredAndEmail(t *testing.T) {
	v := validator.New()
	err := v.Struct(sampleRequest{Name: "", Email: "invalido", Code: "ok"})
	assert.Error(t, err)

	result := pkgerrors.ParseValidationErrors(err)

	assert.NotEmpty(t, result)
	fields := map[string]string{}
	for _, ve := range result {
		fields[ve.Field] = ve.Message
	}
	assert.Contains(t, fields["name"], "obrigatório")
	assert.Contains(t, fields["email"], "inválido")
}

func TestParseValidationErrors_MinMax(t *testing.T) {
	v := validator.New()
	err := v.Struct(sampleRequest{Name: "ok", Email: "a@b.com", Code: "x"})
	assert.Error(t, err)

	result := pkgerrors.ParseValidationErrors(err)

	assert.NotEmpty(t, result)
	found := false
	for _, ve := range result {
		if ve.Field == "code" {
			found = true
			assert.Contains(t, ve.Message, "mínimo")
		}
	}
	assert.True(t, found)
}

func TestParseValidationErrors_MaxAndUnhandledTag(t *testing.T) {
	type withMaxAndCustom struct {
		Code  string `validate:"max=2"`
		Extra string `validate:"gt=5"`
	}

	v := validator.New()
	err := v.Struct(withMaxAndCustom{Code: "abcde", Extra: "1"})
	assert.Error(t, err)

	result := pkgerrors.ParseValidationErrors(err)

	fields := map[string]string{}
	for _, ve := range result {
		fields[ve.Field] = ve.Message
	}
	assert.Contains(t, fields["code"], "máximo")
	// "gt" não tem case dedicado — cai no branch default ("é inválido")
	assert.Contains(t, fields["extra"], "inválido")
}

func TestParseValidationErrors_AllKnownFieldNames(t *testing.T) {
	type fullAddress struct {
		Name    string `validate:"required"`
		CPF     string `validate:"required"`
		CNPJ    string `validate:"required"`
		Email   string `validate:"required"`
		Phone   string `validate:"required"`
		Street  string `validate:"required"`
		Number  string `validate:"required"`
		City    string `validate:"required"`
		State   string `validate:"required"`
		ZipCode string `validate:"required"`
		Other   string `validate:"required"`
	}

	v := validator.New()
	err := v.Struct(fullAddress{})
	assert.Error(t, err)

	result := pkgerrors.ParseValidationErrors(err)
	assert.Len(t, result, 11)

	fields := map[string]string{}
	for _, ve := range result {
		fields[ve.Field] = ve.Message
	}
	// os campos conhecidos ganham um nome amigável em português...
	assert.Contains(t, fields["cpf"], "CPF")
	assert.Contains(t, fields["cnpj"], "CNPJ")
	assert.Contains(t, fields["phone"], "telefone")
	assert.Contains(t, fields["street"], "rua")
	assert.Contains(t, fields["number"], "número")
	assert.Contains(t, fields["city"], "cidade")
	assert.Contains(t, fields["state"], "estado")
	assert.Contains(t, fields["zipcode"], "CEP")
	// ...um campo não mapeado cai no default (usa o próprio nome do campo)
	assert.Contains(t, fields["other"], "Other")
}

func TestParseValidationErrors_NonValidatorError(t *testing.T) {
	err := errors.New("erro genérico, não vindo do validator")

	result := pkgerrors.ParseValidationErrors(err)

	assert.Len(t, result, 1)
	assert.Equal(t, "unknown", result[0].Field)
	assert.Equal(t, "erro genérico, não vindo do validator", result[0].Message)
}
