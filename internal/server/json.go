package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxJSONBodyBytes int64 = 1 << 20 // 1MB

var (
	errInvalidRequestBody  = errors.New("invalid request body")
	errRequestBodyTooLarge = errors.New("request body too large (max 1MB)")
)

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errRequestBodyTooLarge
		}
		return errInvalidRequestBody
	}

	// Reject multiple JSON values in one request body.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errInvalidRequestBody
	}
	return nil
}
