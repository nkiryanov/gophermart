package render

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := map[string]any{"key1": 1, "key2": "222"}
		JSON(w, data)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"key1":1,"key2":"222"}`+"\n", string(body))
}

func TestRender_ServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		message := "something terrible happened"
		ServiceError(w, message, http.StatusForbidden)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.JSONEq(t, `{
			"error": "service_error",
			"message": "something terrible happened"
		}`,
		string(body),
	)
}

func TestRender_DecodeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := struct {
			Key       string `json:"key"`
			OrderName int    `json:"order_name"`
		}{}

		err := json.NewDecoder(r.Body).Decode(&value)
		require.Error(t, err, "Please check what JSON was sent. Test expected that it is invalid")
		DecodeError(w, err)
	}))
	defer ts.Close()

	tests := []struct {
		name        string
		requestBody string
		expected    string
	}{
		{
			name:        "json parsing error",
			requestBody: `invalid-json`,
			expected: `{
				"error":"decoding_failed",
				"message": "Failed to parse JSON: invalid character 'i' looking for beginning of value"
			}`,
		},
		{
			name:        "invalid type ok",
			requestBody: `{"key": "valid_json", "order_name": "but incorrect type"}`,
			expected: `{
				"error": "decoding_failed",
				"message": "Invalid data type for field 'order_name'"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/test", "application/json", strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			defer resp.Body.Close() //nolint:errcheck

			assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
			assert.JSONEq(t, tt.expected, string(body))
		})
	}
}

func TestRender_ValidationErrors(t *testing.T) {
	type T struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"min=6"`
		Email    string `json:"email" validate:"email"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		invalidData := T{
			Password: "123",
			Email:    "not-valid-email",
		}

		err := validate.Struct(invalidData)
		require.Error(t, err, "test expects that data not pass validation")
		errs, ok := err.(validator.ValidationErrors)
		require.True(t, ok, "be sure you pass structure to validator")
		ValidationErrors(w, errs)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	expected, err := json.Marshal(struct {
		Error   string            `json:"error"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	}{
		Error:   "validation_failed",
		Message: "Request validation failed",
		Fields: map[string]string{
			"username": "This field is required",         // Message for 'required' tag
			"password": "Value is too short (minimum 6)", // Message for 'min' validation tag
			"email":    "Invalid value",                  // Unknown validation tag failed: default validation error message
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.JSONEq(t, string(expected), string(body))
}

func TestRender_BindAndValidate(t *testing.T) {
	t.Run("response", func(t *testing.T) {
		type request struct {
			Username string `json:"username" validate:"required"`
			Email    string `json:"email" validate:"email"`
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := BindAndValidate[request](w, r)
			if err != nil {
				return // Error response already written
			}
			// Success case
			JSON(w, map[string]bool{"success": true})
		}))
		defer srv.Close()

		tests := []struct {
			name           string
			requestBody    string
			expectedStatus int
			expectedBody   string
		}{
			{
				name:           "valid request",
				requestBody:    `{"username": "john", "email": "nk@bro.ru"}`,
				expectedStatus: http.StatusOK,
				expectedBody:   `{"success": true}`,
			},
			{
				name:           "invalid json",
				requestBody:    `invalid-json`,
				expectedStatus: http.StatusBadRequest,
				expectedBody: `{
					"error": "decoding_failed",
					"message": "Failed to parse JSON: invalid character 'i' looking for beginning of value"
				}`,
			},
			{
				name:           "field validation fail",
				requestBody:    `{}`,
				expectedStatus: http.StatusUnprocessableEntity,
				expectedBody: `{
					"error": "validation_failed",
					"message": "Request validation failed",
					"fields": {
						"username": "This field is required",
						"email": "Invalid value"
					}
				}`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Post(srv.URL+"/test", "application/json", strings.NewReader(tt.requestBody))
				require.NoError(t, err)
				require.Equal(t, tt.expectedStatus, resp.StatusCode)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck

				assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
				assert.JSONEq(t, tt.expectedBody, string(body))
			})
		}
	})

	t.Run("luhn tag supported", func(t *testing.T) {
		type request struct {
			Number string `json:"number" validate:"luhn"`
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := BindAndValidate[request](w, r)
			if err != nil {
				return // Error response already written
			}
			// Success case
			JSON(w, map[string]bool{"success": true})
		}))
		defer srv.Close()

		tests := []struct {
			name           string
			requestBody    string
			expectedStatus int
			expectedBody   string
		}{
			{
				name:           "valid luhn number",
				requestBody:    `{"number": "17893729974"}`,
				expectedStatus: http.StatusOK,
				expectedBody:   `{"success": true}`,
			},
			{
				name:           "invalid luhn number",
				requestBody:    `{"number": "1234567890"}`,
				expectedStatus: http.StatusUnprocessableEntity,
				expectedBody: `
				{
					"error": "validation_failed",
					"message": "Request validation failed",
					"fields": {
						"number": "Invalid value"
					}
				}`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Post(srv.URL+"/test", "application/json", strings.NewReader(tt.requestBody))
				require.NoError(t, err)
				require.Equal(t, tt.expectedStatus, resp.StatusCode)
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				defer resp.Body.Close() //nolint:errcheck

				require.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
				require.JSONEq(t, tt.expectedBody, string(body))
			})
		}
	})
}
