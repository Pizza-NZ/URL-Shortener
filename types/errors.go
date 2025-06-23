package types

import (
	"fmt"
	"net/http"
	"strings"
)

// Details is a struct used by BadRequestError to provide specific information
// about why a request was invalid.
// This struct was not in the original file, but is required for BadRequestError to compile.
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

// String implements the Stringer interface for Details, providing a formatted
func (d Details) String() string {
	return fmt.Sprintf("%s: %s", d.Field, d.Issue)
}

// --- Specific, Unique Error Types ---
// These errors have a unique structure and don't fit a generic pattern.
// They are kept as-is because their purpose is distinct.

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
	// Use a strings.Builder for more efficient string concatenation in a loop.
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
// This is the core of the refactoring. We create a single, powerful error type
// that can be wrapped to represent different kinds of application errors.

// AppError is a generic error type for the application. It's designed to
// wrap underlying errors while adding context, like an HTTP status code, a
// user-facing message, and an internal (private) message for logging.
type AppError struct {
	// Underlying is the original error that this AppError is wrapping.
	// This is crucial for error chaining and checking with `errors.As` or `errors.Is`.
	Underlying error `json:"-"`

	// HTTPStatus is the HTTP status code that should be returned to the client.
	HTTPStatus int `json:"-"`

	// Message is a safe, user-facing message. It should not contain sensitive info.
	Message string `json:"message"`

	// InternalMessage is a more detailed, developer-facing message for logging
	// and debugging. It can contain technical details.
	InternalMessage string `json:"-"`
}

// Error implements the error interface. It provides a detailed string representation
// of the error, primarily intended for logging.
func (e *AppError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("message: %s, internal_message: %s, underlying_error: %v", e.Message, e.InternalMessage, e.Underlying)
	}
	return fmt.Sprintf("message: %s, internal_message: %s", e.Message, e.InternalMessage)
}

// Unwrap allows for error chaining. It returns the underlying error,
// enabling functions like `errors.Is` and `errors.As` to inspect the entire
// error chain.
func (e *AppError) Unwrap() error {
	return e.Underlying
}

// NewAppError is the constructor for our generic error type. This is the central
// point for creating wrapped errors. Your other error constructors will call this.
func NewAppError(message, internalMessage string, httpStatus int, underlying error) *AppError {
	return &AppError{
		Message:         message,
		InternalMessage: internalMessage,
		HTTPStatus:      httpStatus,
		Underlying:      underlying,
	}
}

// --- Factory Functions for Specific Error Kinds ---
// Instead of creating new structs for DBError, ConfigError, etc., we now create
// simple "factory functions". These functions call NewAppError with predefined
// values to create specific "kinds" of errors, reducing boilerplate code.

// NewDBError creates an AppError specifically for database-related issues.
// It sets a standard user-facing message and HTTP status code.
func NewDBError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"Database operation failed",    // Safe, generic message for the user
		internalMessage,                // Specific details for our logs
		http.StatusInternalServerError, // 500
		underlying,                     // The original error from the database driver
	)
}

// NewConfigError creates an AppError for configuration problems.
func NewConfigError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"Application configuration error",
		internalMessage,
		http.StatusInternalServerError, // 500
		underlying,
	)
}

// NewAuthorizationError creates an AppError for authorization failures (e.g., wrong role).
func NewAuthorizationError(internalMessage string, underlying error) *AppError {
	return NewAppError(
		"You are not authorized to perform this action",
		internalMessage,
		http.StatusForbidden, // 403
		underlying,
	)
}
