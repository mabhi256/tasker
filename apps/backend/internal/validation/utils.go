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
	if err := c.Bind(payload); err != nil {
		return err
	}

	// Now validate the successfully bound values
	var validationErrors []errs.FieldError
	if err := payload.Validate(); err != nil {
		if validationErrs, ok := err.(validator.ValidationErrors); ok {
			for _, verr := range validationErrs {
				source := getFieldSource(payload, verr.Field())
				msg := formatValidationMessage(verr)
				validationErrors = append(validationErrors, createFieldError(verr.Field(), msg, source))
			}
		}
	}

	if len(validationErrors) > 0 {
		return errs.NewUnprocessableError("Validation failed", true, nil, validationErrors, nil)
	}

	return nil
}

func getFieldSource(payload any, fieldName string) string {
	val := reflect.ValueOf(payload).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == fieldName {
			if field.Tag.Get("param") != "" {
				return "param"
			}
			if field.Tag.Get("query") != "" {
				return "query"
			}
			return "json"
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
