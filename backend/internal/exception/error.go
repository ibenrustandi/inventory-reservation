// Package exception defines the errors the service reports to clients.
// It knows nothing about HTTP; the delivery layer decides which status
// code each error gets. It imports nothing else from this project.
package exception

// Error is an error the client is told about. Code is stable and
// machine-readable; clients switch on it.
type Error struct {
	Code    string
	Message string
	Details any
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// Is makes errors.Is match on Code, so an error with details attached
// still matches its base error.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// WithDetails returns a copy of the error carrying the given details.
func (e *Error) WithDetails(details any) *Error {
	c := *e
	c.Details = details
	return &c
}

// Define errors here.
var (
	// inventory domain
	ErrItemNotFound      = &Error{Code: "ITEM_NOT_FOUND", Message: "The item does not exist."}
	ErrInsufficientStock = &Error{Code: "INSUFFICIENT_STOCK", Message: "Not enough stock is available."}

	// reservation domain
	ErrReservationNotFound = &Error{Code: "RESERVATION_NOT_FOUND", Message: "The reservation does not exist."}
	ErrReservationExpired  = &Error{Code: "RESERVATION_EXPIRED", Message: "The reservation has expired."}

	// request
	ErrInvalidRequest   = &Error{Code: "INVALID_REQUEST", Message: "The request is invalid."}
	ErrPayloadTooLarge  = &Error{Code: "PAYLOAD_TOO_LARGE", Message: "The request body is too large."}
	ErrRouteNotFound    = &Error{Code: "NOT_FOUND", Message: "The route does not exist."}
	ErrMethodNotAllowed = &Error{Code: "METHOD_NOT_ALLOWED", Message: "The method is not allowed for this route."}

	// server
	ErrInternalServerError = &Error{Code: "INTERNAL_ERROR", Message: "An unexpected error occurred."}
	ErrServiceUnavailable  = &Error{Code: "SERVICE_UNAVAILABLE", Message: "The service is busy. Try again shortly."}
)

// FieldError names one invalid request field and says what is wrong with
// it. It is listed in the details of ErrInvalidRequest.
type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// NewInvalidRequest returns ErrInvalidRequest listing the invalid fields.
func NewInvalidRequest(fields ...FieldError) *Error {
	return ErrInvalidRequest.WithDetails(map[string]any{"fields": fields})
}
