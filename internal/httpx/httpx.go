// Package httpx provides small helpers for consistent JSON encoding,
// decoding and error responses across all handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Response struct {
	Data any `json:"data,omitempty"`
	Meta any `json:"meta,omitempty"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func Data(w http.ResponseWriter, status int, data any) {
	JSON(w, status, Response{Data: data})
}

func DataMeta(w http.ResponseWriter, status int, data, meta any) {
	JSON(w, status, Response{Data: data, Meta: meta})
}

func Error(w http.ResponseWriter, status int, code, msg string) {
	JSON(w, status, ErrorResponse{Error: ErrorDetail{Code: code, Message: msg}})
}

// Decode reads and decodes a JSON request body, rejecting unknown fields and
// oversized payloads. It returns an error suitable for mapping to HTTP 400.
func Decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func ValidationError(w http.ResponseWriter, msg string) {
	Error(w, http.StatusUnprocessableEntity, "validation_error", msg)
}

func BadRequest(w http.ResponseWriter, msg string) {
	Error(w, http.StatusBadRequest, "bad_request", msg)
}

func NotFound(w http.ResponseWriter, msg string) {
	Error(w, http.StatusNotFound, "not_found", msg)
}

func Conflict(w http.ResponseWriter, msg string) {
	Error(w, http.StatusConflict, "conflict", msg)
}

func Internal(w http.ResponseWriter, msg string) {
	Error(w, http.StatusInternalServerError, "internal_error", msg)
}
