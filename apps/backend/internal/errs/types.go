package errs

import "strings"

type BindError struct {
	Field  *string `json:"field,omitempty"`  // for JSON body fields
	Query  *string `json:"query,omitempty"`  // for query params
	Param  *string `json:"param,omitempty"`  // for path params
	Form   *string `json:"form,omitempty"`   // for form data
	Header *string `json:"header,omitempty"` // for headers
	Error  string  `json:"error"`
}

type ActionType string

const (
	ActionTypeRedirect ActionType = "redirect"
)

type Action struct {
	Type    ActionType `json:"type"`
	Message string     `json:"message"`
	Value   string     `json:"value"`
}

type HTTPError struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Status   int         `json:"status"`
	Override bool        `json:"override"`
	Errors   []BindError `json:"errors"`
	Action   *Action     `json:"action"` // action to be taken
}

func (e *HTTPError) Error() string {
	return e.Message
}

func (e *HTTPError) Is(target error) bool {
	_, ok := target.(*HTTPError)
	return ok
}

func (e *HTTPError) WithMessage(message string) *HTTPError {
	return &HTTPError{
		Code:     e.Code,
		Message:  message,
		Status:   e.Status,
		Override: e.Override,
		Errors:   e.Errors,
		Action:   e.Action,
	}
}

func MakeUpperSnakeCase(str string) string {
	str = strings.ReplaceAll(str, " ", "_")
	str = strings.ReplaceAll(str, "-", "_")

	return strings.ToUpper(str)
}
