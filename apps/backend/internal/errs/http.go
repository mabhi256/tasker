package errs

import "net/http"

func newSimpleError(status int, message string, override bool) *HTTPError {
	return &HTTPError{
		Code:     MakeUpperSnakeCase(http.StatusText(status)),
		Message:  message,
		Status:   status,
		Override: override,
	}
}

func newError(status int, message string, override bool, code *string, errors []FieldError, action *Action) *HTTPError {
	if code == nil {
		formatted := MakeUpperSnakeCase(http.StatusText(status))
		code = &formatted
	}
	return &HTTPError{
		Code:     *code,
		Message:  message,
		Status:   status,
		Override: override,
		Errors:   errors,
		Action:   action,
	}
}

func NewUnauthorizedError(message string, override bool) *HTTPError {
	return newSimpleError(http.StatusUnauthorized, message, override)
}

func NewForbiddenError(message string, override bool) *HTTPError {
	return newSimpleError(http.StatusForbidden, message, override)
}

// Malformed request - bad JSON, wrong types - {"name": "John", "age": }
func NewBadRequestError(message string, override bool, code *string, errors []FieldError, action *Action) *HTTPError {
	return newError(http.StatusBadRequest, message, override, code, errors, action)
}

// Conflicts with existing state
func NewConflictError(message string, override bool, code *string, errors []FieldError, action *Action) *HTTPError {
	return newError(http.StatusConflict, message, override, code, errors, action)
}

func NewNotFoundError(message string, override bool, code *string) *HTTPError {
	return newError(http.StatusNotFound, message, override, code, nil, nil)
}

func NewValidationError(err error) *HTTPError {
	message := "Validation failed: " + err.Error()
	return newError(http.StatusUnprocessableEntity, message, false, nil, nil, nil)
}

// Valid JSON, invalid data (validation/constraint failures) - {"name": "", "age": -5, "email": "notanemail"}
func NewUnprocessableError(message string, override bool, code *string, errors []FieldError, action *Action) *HTTPError {
	return newError(http.StatusUnprocessableEntity, message, false, code, errors, action)
}

func NewInternalServerError() *HTTPError {
	text := http.StatusText(http.StatusInternalServerError)
	return newSimpleError(http.StatusInternalServerError, text, false)
}
