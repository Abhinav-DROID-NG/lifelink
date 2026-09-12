package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
)

// decodeJSON parses a request body into v, enforcing a 1 MiB size limit.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		if isTooLarge(err) {
			writeError(w, ErrBodyTooLarge)
		} else {
			writeError(w, ErrInvalidJSON)
		}
		return err
	}
	return nil
}

func isTooLarge(err error) bool {
	if err == io.EOF {
		return false
	}
	if httpErr, ok := err.(*http.MaxBytesError); ok {
		return httpErr.Limit > 0
	}
	return false
}
