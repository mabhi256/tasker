package validation

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mabhi256/tasker/internal/errs"
)

// CustomBinder collects all binding errors instead of short-circuiting
type CustomBinder struct {
	echo.DefaultBinder
}

func (cb *CustomBinder) Bind(i interface{}, c echo.Context) error {
	var allErrors []errs.FieldError

	// 1. Bind path params and collect errors
	pathErrs := cb.bindParamsWithErrors(c, i, "param")
	allErrors = append(allErrors, pathErrs...)

	// 2. Bind query params and collect errors
	queryErrs := cb.bindParamsWithErrors(c, i, "query")
	allErrors = append(allErrors, queryErrs...)

	// 3. Bind JSON body and collect errors
	bodyErrs, err := cb.bindJSONWithErrors(c, i)
	if err != nil {
		return err // Fatal error (e.g., malformed JSON)
	}
	allErrors = append(allErrors, bodyErrs...)

	if len(allErrors) > 0 {
		return errs.NewUnprocessableError("Validation failed", true, nil, allErrors, nil)
	}

	return nil
}

// bindParamsWithErrors binds params/query and continues on error
func (cb *CustomBinder) bindParamsWithErrors(c echo.Context, i interface{}, source string) []errs.FieldError {
	var errors []errs.FieldError
	rawValues := getRawValues(c, source)

	val := reflect.ValueOf(i).Elem()
	typ := val.Type()

	for idx := 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		structField := val.Field(idx)

		tagValue := field.Tag.Get(source)
		if tagValue == "" || tagValue == "-" || !structField.CanSet() {
			continue
		}

		tagName := strings.Split(tagValue, ",")[0]
		rawValue, exists := rawValues[tagName]

		if !exists || rawValue == "" {
			continue
		}

		// Try to bind - if error, collect it and continue
		if err := bindValue(structField, rawValue); err != nil {
			errors = append(errors, createFieldError(field.Name, err.Error(), source))
			// Continue to next field instead of returning
		}
	}

	return errors
}

// bindJSONWithErrors binds JSON and continues on type errors
func (cb *CustomBinder) bindJSONWithErrors(c echo.Context, i interface{}) ([]errs.FieldError, error) {
	var errors []errs.FieldError

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, echo.NewHTTPError(400, "failed to read request body")
	}

	if len(bodyBytes) == 0 {
		return nil, nil
	}

	var rawMap map[string]any
	if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
		return nil, echo.NewHTTPError(400, "invalid JSON: "+err.Error())
	}

	// Pre-validate structure (unknown fields, type mismatches)
	val := reflect.ValueOf(i).Elem()
	typ := val.Type()

	validFields := make(map[string]reflect.Type)
	for idx := 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		jsonTag := field.Tag.Get("json")

		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		if field.Tag.Get("param") != "" || field.Tag.Get("query") != "" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]
		validFields[fieldName] = field.Type
	}

	// Check unknown fields and type mismatches
	for fieldName, rawValue := range rawMap {
		expectedType, isValid := validFields[fieldName]

		if !isValid {
			errors = append(errors, errs.FieldError{
				Field: &fieldName,
				Error: "unknown field",
			})
			continue
		}

		if expectedType.Kind() == reflect.Ptr {
			expectedType = expectedType.Elem()
		}

		if !isTypeCompatible(expectedType, rawValue) {
			actualType := getJSONType(rawValue)
			expectedStr := getTypeString(expectedType)
			errors = append(errors, errs.FieldError{
				Field: &fieldName,
				Error: fmt.Sprintf("expected %s but got %s", expectedStr, actualType),
			})
		}
	}

	// Still attempt to bind what we can
	// json.Unmarshal will skip fields with type errors
	json.Unmarshal(bodyBytes, i)

	return errors, nil
}

// bindValue converts and binds a string value to the target struct field
func bindValue(structField reflect.Value, rawValue string) error {
	// Handle [16]byte UUID
	if structField.Kind() == reflect.Array && structField.Type().Len() == 16 {
		parsed, err := parseUUID(rawValue)
		if err != nil {
			return fmt.Errorf("invalid UUID format")
		}
		structField.Set(reflect.ValueOf(parsed))
		return nil
	}

	// Handle string
	if structField.Kind() == reflect.String {
		structField.SetString(rawValue)
		return nil
	}

	// Handle *string
	if structField.Kind() == reflect.Ptr && structField.Type().Elem().Kind() == reflect.String {
		structField.Set(reflect.ValueOf(&rawValue))
		return nil
	}

	// Handle int types
	if structField.Kind() >= reflect.Int && structField.Kind() <= reflect.Int64 {
		intVal, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a valid integer")
		}
		structField.SetInt(intVal)
		return nil
	}

	// Handle uint types
	if structField.Kind() >= reflect.Uint && structField.Kind() <= reflect.Uint64 {
		uintVal, err := strconv.ParseUint(rawValue, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a valid unsigned integer")
		}
		structField.SetUint(uintVal)
		return nil
	}

	// Handle float types
	if structField.Kind() == reflect.Float32 || structField.Kind() == reflect.Float64 {
		floatVal, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return fmt.Errorf("must be a valid number")
		}
		structField.SetFloat(floatVal)
		return nil
	}

	// Handle bool
	if structField.Kind() == reflect.Bool {
		boolVal, err := strconv.ParseBool(rawValue)
		if err != nil {
			return fmt.Errorf("must be a valid boolean")
		}
		structField.SetBool(boolVal)
		return nil
	}

	// Handle pointer types
	if structField.Kind() == reflect.Ptr {
		elemType := structField.Type().Elem()
		newElem := reflect.New(elemType).Elem()
		if err := bindValue(newElem, rawValue); err != nil {
			return err
		}
		ptr := reflect.New(elemType)
		ptr.Elem().Set(newElem)
		structField.Set(ptr)
		return nil
	}

	return fmt.Errorf("unsupported type for binding")
}

func getRawValues(c echo.Context, source string) map[string]string {
	values := make(map[string]string)

	switch source {
	case "param":
		names := c.ParamNames()
		paramValues := c.ParamValues()
		for idx, name := range names {
			values[name] = paramValues[idx]
		}
	case "query":
		for key, vals := range c.QueryParams() {
			if len(vals) > 0 {
				values[key] = vals[0]
			}
		}
	}

	return values
}

func createFieldError(fieldName, message, source string) errs.FieldError {
	fieldName = strings.ToLower(fieldName)
	fieldError := errs.FieldError{Error: message}

	switch source {
	case "param":
		fieldError.Param = &fieldName
	case "query":
		fieldError.Query = &fieldName
	case "json":
		fieldError.Field = &fieldName
	}

	return fieldError
}

func parseUUID(s string) ([16]byte, error) {
	var uuid [16]byte
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return uuid, fmt.Errorf("invalid uuid length")
	}

	for i := range 16 {
		_, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &uuid[i])
		if err != nil {
			return uuid, err
		}
	}
	return uuid, nil
}

func getJSONType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func getTypeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct, reflect.Map:
		return "object"
	default:
		return t.String()
	}
}

func isTypeCompatible(expectedType reflect.Type, actualValue any) bool {
	actualType := getJSONType(actualValue)

	switch expectedType.Kind() {
	case reflect.String:
		return actualType == "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return actualType == "number"
	case reflect.Bool:
		return actualType == "boolean"
	case reflect.Slice, reflect.Array:
		return actualType == "array"
	case reflect.Struct, reflect.Map:
		return actualType == "object"
	default:
		return true
	}
}
