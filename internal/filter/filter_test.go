package filter

import (
	"errors"
	"strings"
	"testing"
	"time"

	"log-analyzer/internal/logparse"
)

var testEntry = logparse.Entry{
	IP:       "10.0.0.1",
	User:     "alice",
	Time:     time.Date(2026, 10, 1, 18, 30, 0, 0, time.FixedZone("", -6*60*60)),
	Method:   "GET",
	Path:     "/users/42",
	Protocol: "HTTP/1.1",
	Status:   503,
	Bytes:    1024,
	Referer:  "https://example.com/",
	Agent:    "Googlebot/2.1",
}

func TestMatch(t *testing.T) {
	tests := []struct {
		expr string
		want bool
	}{
		// string equality, every string field
		{`ip == "10.0.0.1"`, true},
		{`user == "alice"`, true},
		{`method == "GET"`, true},
		{`method == "get"`, false}, // case-sensitive
		{`method != "GET"`, false},
		{`method != "POST"`, true},
		{`path == "/users/42"`, true},
		{`protocol == "HTTP/1.1"`, true},
		{`referer == "https://example.com/"`, true},
		{`agent == "Googlebot/2.1"`, true},

		// regex
		{`path =~ "^/users/[0-9]+$"`, true},
		{`path =~ "^/orders"`, false},
		{`path !~ "^/orders"`, true},
		{`path !~ "users"`, false},
		{`agent =~ "(?i)BOT"`, true},
		{`path =~ "^/users/\d+$"`, true},

		// ints
		{`status == 503`, true},
		{`status != 503`, false},
		{`status < 503`, false},
		{`status <= 503`, true},
		{`status > 500`, true},
		{`status >= 504`, false},
		{`bytes == 1024`, true},
		{`bytes > 2000`, false},

		// status classes
		{`status == 5xx`, true},
		{`status == 4xx`, false},
		{`status != 5xx`, false},
		{`status != 2xx`, true},

		// time (the entry is 18:30 at -06:00, i.e. 00:30 UTC the next day)
		{`time == "2026-10-02T00:30:00Z"`, true},
		{`time != "2026-10-02T00:30:00Z"`, false},
		{`time < "2026-10-01T18:30:00-06:00"`, false},
		{`time <= "2026-10-01T18:30:00-06:00"`, true},
		{`time > "2026-10-01T18:00:00-06:00"`, true},
		{`time >= "2026-10-01T19:00:00-06:00"`, false},
		{`time >= "2026-10-01T18:00:00-06:00" && time < "2026-10-01T19:00:00-06:00"`, true},

		// logic, precedence and grouping
		{`status == 5xx && method == "GET"`, true},
		{`status == 5xx && method == "POST"`, false},
		{`status == 4xx || method == "GET"`, true},
		{`status == 4xx || method == "POST"`, false},
		{`!(method == "POST")`, true},
		{`!method == "GET"`, false},
		{`!!(method == "GET")`, true},
		// && binds tighter than ||: true || (false && false)
		{`status == 503 || status == 404 && status == 404`, true},
		// grouping overrides it: (true || false) && false
		{`(status == 503 || status == 404) && status == 404`, false},
		// ! binds tighter than &&: (!false) && false
		{`!status == 404 && status == 404`, false},
		{`path =~ "^/users/[0-9]+$" && !(method == "GET" || method == "HEAD")`, false},
	}

	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			f, err := Compile(tc.expr)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got := f.Match(testEntry); got != tc.want {
				t.Errorf("Match() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantCol int
		wantMsg string // prefix of the message
	}{
		{"empty", ``, 1, "expected field, got end of expression"},
		{"unknown field", `stauts >= 500`, 1, `unknown field "stauts"`},
		{"missing operator", `status`, 7, "expected comparison operator, got end of expression"},
		{"missing value", `status ==`, 10, "expected value, got end of expression"},
		{"field as value", `status == bytes`, 11, "expected value, got field"},
		{"literal on the left", `500 == status`, 1, "expected field, got integer"},
		{"chained comparison", `status == 200 == 300`, 15, `unexpected "=="`},
		{"trailing token", `status == 200 )`, 15, `unexpected ")"`},
		{"unclosed paren", `(status == 200`, 15, `expected ")", got end of expression`},
		{"dangling and", `status == 200 &&`, 17, "expected field, got end of expression"},
		{"string ordering", `method > "GET"`, 8, `operator ">" is not supported for string field "method"`},
		{"int to string", `method == 3`, 11, `cannot compare string field "method" with integer`},
		{"string to int", `status == "500"`, 11, `cannot compare int field "status" with string`},
		{"regex on int", `status =~ "5.."`, 8, `operator "=~" is not supported for int field "status"`},
		{"regex on time", `time =~ "2026"`, 6, `operator "=~" is not supported for time field "time"`},
		{"class on other field", `bytes == 5xx`, 10, "status class can only be compared with status"},
		{"class ordering", `status >= 5xx`, 8, `operator ">=" is not supported for status classes`},
		{"invalid regex", `path =~ "(["`, 9, "invalid regex"},
		{"invalid time", `time > "yesterday"`, 8, `invalid time "yesterday"`},
		{"time without zone", `time > "2026-10-01T18:00:00"`, 8, "invalid time"},
		{"time with int", `time > 5`, 8, `cannot compare time field "time" with integer`},
		{"lexer error", `status = 200`, 8, "unexpected character '='"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.expr)
			var ferr *Error
			if !errors.As(err, &ferr) {
				t.Fatalf("Compile() error = %v, want *Error", err)
			}
			if ferr.Col != tc.wantCol || !strings.HasPrefix(ferr.Msg, tc.wantMsg) {
				t.Errorf("Compile() error = {%d, %q}, want {%d, %q...}", ferr.Col, ferr.Msg, tc.wantCol, tc.wantMsg)
			}
		})
	}
}
