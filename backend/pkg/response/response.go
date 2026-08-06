package response

import (
	"encoding/json"
	"net/http"
)

// Package response provides standard JSON response helpers for the casino API.
//
// All responses use the APIResponse envelope with "success", "data", and "error"
// fields, ensuring a consistent contract for API consumers.
//
// APIResponse is the standard JSON response envelope returned by all API endpoints.
//
// On success, Success is true and Data contains the payload. On error, Success is
// false and Error contains the human-readable message. The omitempty tags ensure
// that Data is omitted on errors and Error is omitted on successes.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON writes a JSON response with the given HTTP status code. The "success" field
// is automatically set to true for 2xx status codes. The Content-Type header is
// set to application/json. If encoding fails the error is silently dropped — this
// is typical for handler-level helpers where the connection may already be closed.
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success: statusCode >= 200 && statusCode < 300,
		Data:    data,
	}

	json.NewEncoder(w).Encode(resp)
}

// Error writes a JSON error response with the given HTTP status code and
// human-readable message. The "success" field is always false. The Content-Type
// header is set to application/json.
func Error(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(resp)
}
