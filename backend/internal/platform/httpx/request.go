package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rootlogic-lab/delivery/backend/internal/shared/errs"
)

// maxBodyBytes caps a request body at 1 MiB.
//
// Without a cap, a single request can make the process allocate until it dies;
// with one, an oversized body is a 400 for that caller and nothing else. No
// endpoint in this API legitimately sends more, and file upload will use a
// separate, explicitly larger path.
const maxBodyBytes = 1 << 20

// DecodeJSON reads a JSON body into v, rejecting anything malformed.
//
// Unknown fields are an error rather than silently ignored: a client sending
// "quantiy" instead of "quantity" should be told, not have its value dropped
// and the order placed wrong.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return errs.New(errs.KindInvalid, "unsupported_media_type",
			"This endpoint accepts JSON only.")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return decodeError(err)
	}
	// A second value in the stream means the client sent two documents; taking
	// only the first would silently ignore the rest.
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return errs.New(errs.KindInvalid, "malformed_json",
			"The request body must contain a single JSON object.")
	}
	return nil
}

// decodeError turns a decoder failure into a message that tells the caller
// what to fix. json's own errors name Go types, which mean nothing to a client.
func decodeError(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return errs.Wrap(err, errs.KindInvalid, "request_too_large",
			"That request was too large.")
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return errs.Wrap(err, errs.KindInvalid, "invalid_field_type",
			"A field in the request has the wrong type.").
			With("field", typeErr.Field)
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return errs.Wrap(err, errs.KindInvalid, "malformed_json",
			"The request body is not valid JSON.")
	}
	if errors.Is(err, io.EOF) {
		return errs.Wrap(err, errs.KindInvalid, "empty_body",
			"The request body is required.")
	}
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return errs.Wrap(err, errs.KindInvalid, "unknown_field",
			"The request contains a field this endpoint does not accept.").
			With("field", field)
	}
	return errs.Wrap(err, errs.KindInvalid, "malformed_json",
		"The request body could not be read.")
}

// RequiredFloat reads a required float query parameter.
func RequiredFloat(r *http.Request, key string) (float64, error) {
	raw := r.URL.Query().Get(key)
	if strings.TrimSpace(raw) == "" {
		return 0, errs.New(errs.KindInvalid, "missing_parameter",
			"A required parameter is missing.").With("parameter", key)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errs.Wrap(err, errs.KindInvalid, "invalid_parameter",
			"A parameter is not a number.").With("parameter", key)
	}
	return v, nil
}

// OptionalInt reads an integer query parameter, falling back to def.
func OptionalInt(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errs.Wrap(err, errs.KindInvalid, "invalid_parameter",
			"A parameter is not a whole number.").With("parameter", key)
	}
	return v, nil
}

// BoundedInt reads an optional integer and clamps it into [minimum, maximum].
//
// Clamping rather than rejecting is deliberate for page sizes: a client asking
// for 10,000 results wants "as many as you'll give me", and answering with the
// maximum is more useful than a 400. A value below the minimum is a different
// matter — it is a bug in the caller, and silently returning the minimum would
// hide it.
func BoundedInt(r *http.Request, key string, def, minimum, maximum int) (int, error) {
	v, err := OptionalInt(r, key, def)
	if err != nil {
		return 0, err
	}
	if v < minimum {
		return 0, errs.New(errs.KindInvalid, "invalid_parameter",
			"A parameter is below the smallest allowed value.").
			With("parameter", key).
			With("minimum", strconv.Itoa(minimum))
	}
	if v > maximum {
		return maximum, nil
	}
	return v, nil
}
