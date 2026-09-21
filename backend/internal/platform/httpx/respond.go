// Package httpx is the shared HTTP transport layer.
//
// Every module's transport package uses it, so one decision about how an error
// reaches a client is made once here rather than differently in fifteen
// handlers. Nothing in it knows about any module's domain.
//
// The error envelope matches components.schemas.Error in api/openapi.yaml.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// ErrorBody is the wire form of a failure.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// envelope wraps an error so a client can branch on the shape of the response
// body alone, without inspecting the status code first.
type envelope struct {
	Error ErrorBody `json:"error"`
}

// WriteJSON writes a value as JSON with the given status.
//
// The body is marshalled before the status is written: a value that fails to
// marshal would otherwise produce a 200 followed by a truncated body, which
// looks to a client like a successful but corrupt response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		WriteError(w, errs.Wrap(err, errs.KindInternal, "response_encoding_failed",
			"Something went wrong on our side."))
		return
	}
	// The charset is explicit because it has to be. JSON is UTF-8 by
	// definition (RFC 8259 s8.1), but Dart's package:http decodes a body with
	// no declared charset as latin1 — which turns every Bengali sentence this
	// server composes into mojibake on the client that reads it.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteNoContent answers with 204.
func WriteNoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// StatusFor maps an error kind to an HTTP status.
//
// This is the only place the mapping exists. A handler that wants a different
// status returns a different Kind rather than writing a status itself, so the
// API cannot drift into returning 404 for one missing thing and 400 for
// another.
func StatusFor(kind errs.Kind) int {
	switch kind {
	case errs.KindInvalid:
		return http.StatusBadRequest
	case errs.KindUnauthorized:
		return http.StatusUnauthorized
	case errs.KindForbidden:
		return http.StatusForbidden
	case errs.KindNotFound:
		return http.StatusNotFound
	case errs.KindConflict:
		return http.StatusConflict
	case errs.KindRateLimited:
		return http.StatusTooManyRequests
	case errs.KindUnavailable:
		return http.StatusServiceUnavailable
	case errs.KindInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// internalMessage is what a client is told about an unexpected failure.
//
// An internal error's own message may name a table, a column or a connection
// string. None of that is the caller's business, and all of it is useful to an
// attacker, so the message is replaced rather than passed through. The real
// error still reaches the logs via the Recover and Logging middleware.
const internalMessage = "Something went wrong on our side. Please try again."

// WriteError writes an error in the standard envelope.
func WriteError(w http.ResponseWriter, err error) {
	kind := errs.KindOf(err)
	body := ErrorBody{
		Code:    errs.CodeOf(err),
		Message: errs.MessageOf(err),
	}
	if kind == errs.KindInternal {
		body.Message = internalMessage
		body.Details = nil
	} else {
		var e *errs.Error
		if errors.As(err, &e) {
			if fields := e.Fields(); len(fields) > 0 {
				body.Details = fields
			}
		}
	}
	WriteJSON(w, StatusFor(kind), envelope{Error: body})
}
