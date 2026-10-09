// Package model holds the request and response structures of the API.
package model

// ErrorResponse is the single envelope used for every error.
type ErrorResponse struct {
	Status    string    `json:"status"`
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id"`
}

// ErrorBody carries a stable machine-readable code, a message for people
// and optional details that depend on the code.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// FieldError names one invalid request field.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// StatusResponse is the body of the health endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}
