package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/http"
	"reflect"
	"strings"
)

const (
	ValidationErrorType = "validation_failed"
	DecodingErrorType   = "decoding_failed"
	ServiceErrorType    = "service_error"
)

var validate = validator.New()

func init() {
	useJSONTagNames := func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		// skip if tag key says it should be ignored
		if name == "-" {
			return ""
		}
		return name
	}

	validate.RegisterTagNameFunc(useJSONTagNames)
}

type Struct any

type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func JSON(w http.ResponseWriter, data any) {
	JSONWithStatus(w, data, http.StatusOK)
}

// renderJSONWithStatus sends data as json and enforces status code
func JSONWithStatus(w http.ResponseWriter, data any, code int) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)

	if err := enc.Encode(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

// BindAndValidate decodes JSON request body into type T and validates it using struct tags.
// Returns the decoded value and writes appropriate error responses for decoding or validation failures.
func BindAndValidate[T Struct](w http.ResponseWriter, r *http.Request) (T, error) {
	var value T

	err := json.NewDecoder(r.Body).Decode(&value)
	if err != nil {
		decodeError(w, err)
		return value, err
	}

	return value, ValidateStruct(w, value)
}

// Take struct and run validator for it
// If validation fails, it writes validation errors to response
func ValidateStruct(w http.ResponseWriter, v any) error {
	err := validate.Struct(v)
	if err != nil {
		validationErrors(w, err.(validator.ValidationErrors))
		return err
	}
	return nil
}

// Render error message as service error
func ServiceError(w http.ResponseWriter, error string, code int) {
	response := ErrorResponse{
		Error:   ServiceErrorType,
		Message: error,
	}

	JSONWithStatus(w, response, code)
}

// Render json DecodeError
func decodeError(w http.ResponseWriter, err error) {
	response := ErrorResponse{
		Error:   DecodingErrorType,
		Message: "",
	}

	// Try to provide more specific error message based on error type
	switch err := err.(type) {
	case *json.UnmarshalTypeError:
		response.Message = fmt.Sprintf("Invalid data type for field '%s'", err.Field)
	default:
		response.Message = fmt.Sprintf("Failed to parse JSON: %s", err.Error())
	}

	JSONWithStatus(w, response, http.StatusBadRequest)
}

// Render ValidationErrors
func validationErrors(w http.ResponseWriter, errs validator.ValidationErrors) {
	response := ErrorResponse{
		Error:   ValidationErrorType,
		Message: "Request validation failed",
		Fields:  make(map[string]string, len(errs)),
	}

	// Create user-friendly error messages based on validation tag
	for _, fieldError := range errs {
		var message string
		switch fieldError.Tag() {
		case "required":
			message = "This field is required"
		case "min":
			message = fmt.Sprintf("Value is too short (minimum %s)", fieldError.Param())
		case "luhn":
			message = "Invalid value according to Luhn algorithm"
		default:
			message = "Invalid value"
		}

		response.Fields[fieldError.Field()] = message
	}

	JSONWithStatus(w, response, http.StatusUnprocessableEntity)
}
