package response

import (
	"encoding/json"
	"net/http"
)

// envelope is the standard response shape for every API response.
// Success: { "success": true,  "data": <payload> }
// Error:   { "success": false, "error": "<message>" }
type envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// Meta carries pagination information for list responses.
type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// JSON writes a JSON response with the given status code and body.
func JSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// Success writes a 200 OK response with the given data payload.
func Success(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, envelope{Success: true, Data: data})
}

// Created writes a 201 Created response with the given data payload.
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, envelope{Success: true, Data: data})
}

// Paginated writes a 200 OK response with data and pagination metadata.
func Paginated(w http.ResponseWriter, data interface{}, meta Meta) {
	JSON(w, http.StatusOK, envelope{Success: true, Data: data, Meta: &meta})
}

// NoContent writes a 204 No Content response (e.g. after a DELETE).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Error writes a JSON error response with the given status code and message.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, envelope{Success: false, Error: message})
}

// BadRequest writes a 400 response.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, message)
}

// Unauthorized writes a 401 response.
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, message)
}

// Forbidden writes a 403 response.
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, message)
}

// NotFound writes a 404 response.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, message)
}

// UnprocessableEntity writes a 422 response — used for validation errors.
func UnprocessableEntity(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnprocessableEntity, message)
}

// InternalServerError writes a 500 response.
// Never expose internal error details to the client — log them server-side instead.
func InternalServerError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "an unexpected error occurred")
}