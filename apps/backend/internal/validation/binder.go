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

const bindingErrorsKey = "binding_errors"

type CustomBinder struct {
	echo.DefaultBinder
}

func (cb *CustomBinder) Bind(i any, c echo.Context) error {
	var allErrors []errs.BindError

	// Bind params (path + query + form + header)
	paramErrs := cb.BindParams(c, i)
	allErrors = append(allErrors, paramErrs...)

	// Bind JSON body
	bodyErrs, err := cb.BindBody(c, i)
	if err != nil {
		return err // malformed JSON
	}
	allErrors = append(allErrors, bodyErrs...)

	if len(allErrors) > 0 {
		c.Set(bindingErrorsKey, allErrors)
	}
	return nil
}

// BindParams handles path, query, form and header params
func (cb *CustomBinder) BindParams(c echo.Context, i any) []errs.BindError {
	var errors []errs.BindError

	val := reflect.ValueOf(i).Elem()
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !fieldVal.CanSet() {
			continue
		}

		for _, source := range []string{"param", "query", "form", "header"} {
			if rawValue := cb.getParamValue(c, field, source); rawValue != "" {
				if err := bindValue(fieldVal, rawValue); err != nil {
					errors = append(errors, createFieldError(field.Name, err.Error(), source))
				}
				break
			}
		}
	}

	return errors
}

func (cb *CustomBinder) getParamValue(c echo.Context, field reflect.StructField, source string) string {
	tag := field.Tag.Get(source)
	if tag == "" || tag == "-" {
		return ""
	}

	tagName := strings.Split(tag, ",")[0]

	switch source {
	case "param":
		return c.Param(tagName)
	case "query":
		return c.QueryParam(tagName)
	case "form":
		return c.FormValue(tagName)
	case "header":
		return c.Request().Header.Get(tagName)
	default:
		return ""
	}
}

// BindBody validates types, checks unknown fields, then unmarshals
func (cb *CustomBinder) BindBody(c echo.Context, i any) ([]errs.BindError, error) {
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

	// Validate structure
	errors := cb.validateJSONStructure(i, rawMap)

	// Still unmarshal what we can (ignoring errors since we already validated)
	json.Unmarshal(bodyBytes, i)

	return errors, nil
}

func (cb *CustomBinder) validateJSONStructure(i any, rawMap map[string]any) []errs.BindError {
	var errors []errs.BindError

	// Build expected fields map
	validFields := cb.getJSONFields(i)

	// Check each field in the incoming JSON
	for fieldName, rawValue := range rawMap {
		expectedType, exists := validFields[fieldName]

		if !exists {
			errors = append(errors, errs.BindError{
				Field: &fieldName,
				Error: "unknown field",
			})
			continue
		}

		// Check type compatibility
		if !isTypeCompatible(expectedType, rawValue) {
			actualType := getJSONType(rawValue)
			expectedStr := getTypeString(expectedType)
			errors = append(errors, errs.BindError{
				Field: &fieldName,
				Error: fmt.Sprintf("expected %s but got %s", expectedStr, actualType),
			})
		}
	}

	return errors
}

func (cb *CustomBinder) getJSONFields(i any) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	typ := reflect.TypeOf(i).Elem()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip non-JSON fields
		if field.Tag.Get("param") != "" ||
			field.Tag.Get("query") != "" ||
			field.Tag.Get("form") != "" ||
			field.Tag.Get("header") != "" {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		fieldName := strings.Split(jsonTag, ",")[0]
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}
		fields[fieldName] = fieldType
	}

	return fields
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

func createFieldError(fieldName, message, source string) errs.BindError {
	fieldName = strings.ToLower(fieldName)
	fieldError := errs.BindError{Error: message}

	switch source {
	case "param":
		fieldError.Param = &fieldName
	case "query":
		fieldError.Query = &fieldName
	case "form":
		fieldError.Form = &fieldName
	case "header":
		fieldError.Header = &fieldName
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
