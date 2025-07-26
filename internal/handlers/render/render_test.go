package render

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	assert.JSONEq(t, `{"key1": 1, "key2": "222"}`+"\n", string(body))
}

func TestRender_JSONWithStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := map[string]any{"key1": 1}
		JSONWithStatus(w, data, http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck

	assert.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.JSONEq(t, `{"key1": 1}`+"\n", string(body))
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
}
