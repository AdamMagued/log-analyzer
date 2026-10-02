// Package report renders log entries as list output or as a summary, in text
// or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"log-analyzer/internal/logparse"
)

// WriteListText writes the entry's raw log line.
func WriteListText(w io.Writer, e logparse.Entry) error {
	_, err := fmt.Fprintln(w, e.Raw)
	return err
}

// listEntryJSON fixes the field names and order of a JSON Lines entry.
type listEntryJSON struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	IP       string `json:"ip"`
	User     string `json:"user"`
	Time     string `json:"time"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Protocol string `json:"protocol"`
	Status   int    `json:"status"`
	Bytes    int    `json:"bytes"`
	Referer  string `json:"referer"`
	Agent    string `json:"agent"`
	Raw      string `json:"raw"`
}

// WriteListJSON writes the entry as one JSON Lines object. file and line
// identify where the entry came from.
func WriteListJSON(w io.Writer, file string, line int, e logparse.Entry) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(listEntryJSON{
		File:     file,
		Line:     line,
		IP:       e.IP,
		User:     e.User,
		Time:     e.Time.Format(time.RFC3339),
		Method:   e.Method,
		Path:     e.Path,
		Protocol: e.Protocol,
		Status:   e.Status,
		Bytes:    e.Bytes,
		Referer:  e.Referer,
		Agent:    e.Agent,
		Raw:      e.Raw,
	})
}
