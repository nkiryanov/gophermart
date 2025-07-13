package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/http"
)

const (
	ValidationErrorType = "validation_failed"
	DecodingErrorType   = "decoding_failed"
	ServiceErrorType    = "service_error"
)

var validate = validator.New()

type Struct any

type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// BindAndValidate decodes JSON request body into type T and validates it using struct tags.
// Returns the decoded value and writes appropriate error responses for decoding or validation failures.
func BindAndValidate[T Struct](w http.ResponseWriter, r *http.Request) (T, error) {
	var value T

	err := json.NewDecoder(r.Body).Decode(&value)
	if err != nil {
		WriteDecodingError(w, err)
		return value, err
	}

	err = validate.Struct(value)
	if err != nil {
		// pretty sure cast will be ok cause expecting T is valid struct
		errs := err.(validator.ValidationErrors)
		WriteValidationError(w, errs)
		return value, err
	}

	return value, nil
}

func JSON(w http.ResponseWriter, data any) {
	renderJSONWithStatus(w, data, http.StatusOK)
}

// WriteValidationError writes validation errors in beautiful JSON format
func WriteValidationError(w http.ResponseWriter, errs validator.ValidationErrors) {
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
		case "max":
			message = fmt.Sprintf("Value is too long (maximum %s)", fieldError.Param())
		default:
			message = "Invalid value"
		}

		response.Fields[fieldError.Field()] = message
	}

	renderJSONWithStatus(w, response, http.StatusBadRequest)
}

// WriteDecodingError writes JSON decoding errors in beautiful format
func WriteDecodingError(w http.ResponseWriter, err error) {
	response := ErrorResponse{
		Error:   DecodingErrorType,
		Message: "",
	}

	// Try to provide more specific error message based on error type
	switch err := err.(type) {
	case *json.UnmarshalTypeError:
		response.Message = "Invalid data type for field '" + err.Field + "'"
	default:
		response.Message = "Failed to parse JSON: " + err.Error()
	}

	renderJSONWithStatus(w, response, http.StatusBadRequest)
}

func WriteServiceError(w http.ResponseWriter, error string, code int) {
	response := ErrorResponse{
		Error:   ServiceErrorType,
		Message: error,
	}

	renderJSONWithStatus(w, response, code)
}

// renderJSONWithStatus sends data as json and enforces status code
func renderJSONWithStatus(w http.ResponseWriter, data interface{}, code int) {
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
