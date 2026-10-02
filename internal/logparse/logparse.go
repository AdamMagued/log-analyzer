// Package logparse parses Apache access log lines in Common Log Format (CLF)
// and Combined Log Format.
package logparse

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeLayout is the layout of the %t field, without the surrounding brackets.
const TimeLayout = "02/Jan/2006:15:04:05 -0700"

// ErrMalformed is returned for lines that match neither supported format.
var ErrMalformed = errors.New("malformed log line")

// Entry is one parsed log line.
type Entry struct {
	IP       string
	User     string // "" when the log has "-"
	Time     time.Time
	Method   string
	Path     string // as it appears in the log, not URL-decoded
	Protocol string
	Status   int
	Bytes    int    // 0 when the log has "-"
	Referer  string // "" for CLF lines
	Agent    string // "" for CLF lines
	Raw      string
}

// lineRE matches CLF, optionally followed by the Combined referer and agent.
var lineRE = regexp.MustCompile(
	`^(\S+) \S+ (\S+) \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-)` +
		`(?: "((?:[^"\\]|\\.)*)" "((?:[^"\\]|\\.)*)")?$`)

// Parse parses a single log line. Errors wrap ErrMalformed.
func Parse(line string) (Entry, error) {
	m := lineRE.FindStringSubmatch(line)
	if m == nil {
		return Entry{}, ErrMalformed
	}

	t, err := time.Parse(TimeLayout, m[3])
	if err != nil {
		return Entry{}, fmt.Errorf("%w: invalid timestamp %q", ErrMalformed, m[3])
	}

	req := strings.Split(m[4], " ")
	if len(req) != 3 || req[0] == "" || req[1] == "" || req[2] == "" {
		return Entry{}, fmt.Errorf("%w: invalid request line %q", ErrMalformed, m[4])
	}

	status, _ := strconv.Atoi(m[5]) // the regexp guarantees three digits
	if status < 100 || status > 599 {
		return Entry{}, fmt.Errorf("%w: invalid status %d", ErrMalformed, status)
	}

	bytes := 0
	if m[6] != "-" {
		bytes, err = strconv.Atoi(m[6])
		if err != nil {
			return Entry{}, fmt.Errorf("%w: invalid bytes %q", ErrMalformed, m[6])
		}
	}

	return Entry{
		IP:       m[1],
		User:     dashToEmpty(m[2]),
		Time:     t,
		Method:   req[0],
		Path:     req[1],
		Protocol: req[2],
		Status:   status,
		Bytes:    bytes,
		Referer:  m[7],
		Agent:    m[8],
		Raw:      line,
	}, nil
}

func dashToEmpty(s string) string {
	if s == "-" {
		return ""
	}
	return s
}
