package types

import (
	"fmt"
	"net/http"
	"strings"
)

// Details is a struct used by BadRequestError to provide specific information
// about why a request was invalid.
type Details struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// NewDetails creates a new Details instance with the specified field and issue.
func NewDetails(field, issue string) Details {
	return Details{
		Field: field,
		Issue: issue,
	}
}

// String implements the Stringer interface for Details, providing a formatted string.
func (d Details) String() string {
	return fmt.Sprintf("%s: %s", d.Field, d.Issue)
}

// --- Specific, Unique Error Types ---

// NotFoundError is used when a specific item (identified by a key) cannot be found.
type NotFoundError struct {
	key string
}

// Error implements the error interface for NotFoundError.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("the requested key (%s) does not exist", e.key)
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(key string) *NotFoundError {
	return &NotFoundError{key: key}
}

// BadRequestError is used for validation errors, providing detailed feedback
// on which fields were incorrect.
type BadRequestError struct {
	Details []Details `json:"details"`
}

// Error implements the error interface for BadRequestError.
func (e *BadRequestError) Error() string {
	if len(e.Details) == 0 {
		return "bad request with no details provided"
	}
	var sb strings.Builder
	sb.WriteString("bad request with issues: ")
	for i, detail := range e.Details {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(detail.String())
	}
	return sb.String()
}

// NewBadRequestError creates a new BadRequestError with a slice of details.
func NewBadRequestError(details []Details) *BadRequestError {
	return &BadRequestError{
		Details: details,
	}
}

// --- Generic, Reusable Error Infrastructure ---

// AppError is a generic error type for the application.
// It wraps underlying errors while adding context like an HTTP status code and user-facing messages.
type AppError struct {
	Underlying      error `json:"-"`
	HTTPStatus      int    `json:"-"`
	Message         string `json:"message"`
	InternalMessage string `json:"-"`
}

// Error implements the error interface, providing a detailed string representation for logging.
func (e *AppError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("message: %s, internal_message: %s, underlying_error: %v", e.Message, e.InternalMessage, e.Underlying)
	}
	return fmt.Sprintf("message: %s, internal_message: %s", e.Message, e.InternalMessage)
}

// Unwrap allows for error chaining by returning the underlying error.
func (e *AppError) Unwrap() error {
	return e.Underlying
}

// NewAppError is the constructor for the generic AppError type.
func NewAppError(message, internalMessage string, httpStatus int, underlying error) *AppError {
	return &AppError{
		Message:         message,
		InternalMessage: internalMessage,
		HTTPStatus:      httpStatus,
		Underlying:      underlying,
	}
}

// --- Factory Functions for Specific Error Kinds ---

// NewDBError creates an AppError specifically for database-related issues.
func NewDBError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"Database operation failed",
		internalMessage,
		http.StatusInternalServerError,
		underlying,
	)
}

// NewConfigError creates an AppError for configuration problems.
func NewConfigError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"Application configuration error",
		internalMessage,
		http.StatusInternalServerError,
		underlying,
	)
}

// NewAuthorizationError creates an AppError for authorization failures.
func NewAuthorizationError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"You are not authorized to perform this action",
		internalMessage,
		http.StatusForbidden,
		underlying,
	)
}