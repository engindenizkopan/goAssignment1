package transporthttp

import (
	"encoding/json"
	"net/http"
)

type Problem struct {
	Type     string              `json:"type,omitempty"`
	Title    string              `json:"title,omitempty"`
	Status   int                 `json:"status,omitempty"`
	Detail   string              `json:"detail,omitempty"`
	Instance string              `json:"instance,omitempty"`
	Errors   map[string][]string `json:"errors,omitempty"`
	Meta     map[string]any      `json:"meta,omitempty"`
}

func WriteProblem(w http.ResponseWriter, status int, title, detail string, errs map[string][]string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Problem{
		Title:  title,
		Status: status,
		Detail: detail,
		Errors: errs,
	})
}
