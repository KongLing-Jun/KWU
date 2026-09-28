package serve

import (
	"net/http"
	"strings"
)

type submitRequest struct {
	Input    string `json:"input"`
	Timezone string `json:"timezone,omitempty"`
}

func (s *Server) decodeSubmitRequest(w http.ResponseWriter, r *http.Request) (submitRequest, bool) {
	var body submitRequest
	if err := decodeJSONRequest(w, r, &body); err != nil || body.Input == "" {
		writeJSONRequestError(w, err, "missing input")
		return submitRequest{}, false
	}
	if timezone := strings.TrimSpace(body.Timezone); timezone != "" {
		if err := s.ctl().SetTimezone(timezone); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return submitRequest{}, false
		}
	}
	return body, true
}
