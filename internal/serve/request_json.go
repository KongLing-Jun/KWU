package serve

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxJSONRequestBodyBytes int64 = 1 << 20

func decodeJSONRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONRequestBodyBytes)).Decode(dst)
}

func writeJSONRequestError(w http.ResponseWriter, err error, fallback string) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, fallback, http.StatusBadRequest)
}
