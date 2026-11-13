package dto

type DiagnosisRequest struct {
	Diagnosis string `json:"diagnosis" binding:"required"`
}
