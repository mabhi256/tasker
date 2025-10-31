package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/mabhi256/tasker/internal/errs"
)

type Validatable interface {
	Validate() error
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func IsValidUUID(uuid string) bool {
	return uuidRegex.MatchString(uuid)
}

func BindAndValidate(c echo.Context, payload Validatable) error {
	var allErrors []errs.BindError

	// Uses CustomBinder as defined in the router
	if err := c.Bind(payload); err != nil {
		// Only HTTP errors (400) should be returned immediately
		if httpErr, ok := err.(*echo.HTTPError); ok {
			return httpErr
		}
	}

	// Retrieve any binding errors from context
	fieldsWithBindingErrors := make(map[string]bool)
	if bindingErrs, ok := c.Get(bindingErrorsKey).([]errs.BindError); ok {
		allErrors = append(allErrors, bindingErrs...)

		// Track which fields have binding errors
		for _, bindErr := range bindingErrs {
			if bindErr.Param != nil {
				fieldsWithBindingErrors[*bindErr.Param] = true
			}
			if bindErr.Query != nil {
				fieldsWithBindingErrors[*bindErr.Query] = true
			}
			if bindErr.Field != nil {
				fieldsWithBindingErrors[*bindErr.Field] = true
			}
			if bindErr.Form != nil {
				fieldsWithBindingErrors[*bindErr.Form] = true
			}
			if bindErr.Header != nil {
				fieldsWithBindingErrors[*bindErr.Header] = true
			}
		}
	}

	// Now validate the successfully bound values
	if err := payload.Validate(); err != nil {
		if valErrors, ok := err.(validator.ValidationErrors); ok {
			for _, valErr := range valErrors {
				fieldName := strings.ToLower(valErr.Field())

				// Skip validation error if this field already has a binding error
				if fieldsWithBindingErrors[fieldName] {
					continue
				}

				source := getFieldSource(payload, valErr.Field())
				msg := formatValidationMessage(valErr)
				allErrors = append(allErrors, createFieldError(valErr.Field(), msg, source))
			}
		}
	}

	if len(allErrors) > 0 {
		return errs.NewUnprocessableError("Validation failed", true, nil, allErrors, nil)
	}

	return nil
}

func getFieldSource(payload any, fieldName string) string {
	val := reflect.ValueOf(payload).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == fieldName {
			switch {
			case field.Tag.Get("param") != "":
				return "param"
			case field.Tag.Get("query") != "":
				return "query"
			case field.Tag.Get("form") != "":
				return "form"
			case field.Tag.Get("header") != "":
				return "header"
			default:
				return "json"
			}
		}
	}
	return "json"
}

func formatValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "min":
		if err.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", err.Param())
		}
		return fmt.Sprintf("must be at least %s", err.Param())
	case "max":
		if err.Type().Kind() == reflect.String {
			return fmt.Sprintf("must not exceed %s characters", err.Param())
		}
		return fmt.Sprintf("must not exceed %s", err.Param())
	case "oneof":
		return fmt.Sprintf("must be one of: %s", err.Param())
	case "email":
		return "must be a valid email address"
	case "e164":
		return "must be a valid phone number with country code"
	case "uuid":
		return "must be a valid UUID"
	case "uuidList":
		return "must be a comma-separated list of valid UUIDs"
	default:
		if err.Param() != "" {
			return fmt.Sprintf("%s: %s:%s", strings.ToLower(err.Field()), err.Tag(), err.Param())
		}
		return fmt.Sprintf("%s: %s", strings.ToLower(err.Field()), err.Tag())
	}
}
