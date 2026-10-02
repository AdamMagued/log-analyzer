package logparse

import (
	"errors"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	ts := time.Date(2026, 10, 1, 18, 56, 2, 0, time.FixedZone("", -6*60*60))

	tests := []struct {
		name string
		line string
		want Entry
	}{
		{
			name: "CLF with user",
			line: `45.146.85.105 - reichert8708 [01/Oct/2026:18:56:02 -0600] "GET /24%2f7/morph HTTP/1.1" 400 29820`,
			want: Entry{IP: "45.146.85.105", User: "reichert8708", Time: ts, Method: "GET",
				Path: "/24%2f7/morph", Protocol: "HTTP/1.1", Status: 400, Bytes: 29820},
		},
		{
			name: "CLF with dash user and bytes",
			line: `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "DELETE /users/7?force=true HTTP/2.0" 204 -`,
			want: Entry{IP: "10.0.0.1", Time: ts, Method: "DELETE",
				Path: "/users/7?force=true", Protocol: "HTTP/2.0", Status: 204},
		},
		{
			name: "Combined",
			line: `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200 512 "https://example.com/" "curl/8.5.0"`,
			want: Entry{IP: "10.0.0.1", Time: ts, Method: "GET", Path: "/users", Protocol: "HTTP/1.1",
				Status: 200, Bytes: 512, Referer: "https://example.com/", Agent: "curl/8.5.0"},
		},
		{
			name: "Combined with empty referer and escaped quote in agent",
			line: `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200 512 "-" "bot \"x\""`,
			want: Entry{IP: "10.0.0.1", Time: ts, Method: "GET", Path: "/users", Protocol: "HTTP/1.1",
				Status: 200, Bytes: 512, Referer: "-", Agent: `bot \"x\"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.line)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			tc.want.Raw = tc.line
			if !got.Time.Equal(tc.want.Time) {
				t.Errorf("Time = %v, want %v", got.Time, tc.want.Time)
			}
			if _, off := got.Time.Zone(); off != -6*60*60 {
				t.Errorf("offset = %d, want %d", off, -6*60*60)
			}
			got.Time, tc.want.Time = time.Time{}, time.Time{}
			if got != tc.want {
				t.Errorf("Parse() =\n  %+v\nwant\n  %+v", got, tc.want)
			}
		})
	}
}

func TestParseMalformed(t *testing.T) {
	tests := []struct {
		name string
		line string
	}{
		{"empty", ""},
		{"garbage", "this is not a log line"},
		{"truncated", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200`},
		{"bad timestamp", `10.0.0.1 - - [32/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" 200 1`},
		{"request line with two parts", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /" 200 1`},
		{"request line with four parts", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1 x" 200 1`},
		{"non-numeric status", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" abc 1`},
		{"status out of range", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" 600 1`},
		{"non-numeric bytes", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" 200 1k`},
		{"trailing garbage", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" 200 1 extra`},
		{"only referer", `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET / HTTP/1.1" 200 1 "-"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.line)
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("Parse() error = %v, want ErrMalformed", err)
			}
		})
	}
}
