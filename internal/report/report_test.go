package report

import (
	"bytes"
	"testing"
	"time"

	"log-analyzer/internal/logparse"
)

var (
	mdt = time.FixedZone("", -6*60*60)
	t0  = time.Date(2026, 10, 1, 18, 0, 0, 0, mdt)
)

func entry(status int, at time.Time) logparse.Entry {
	return logparse.Entry{Status: status, Time: at}
}

func TestWriteListText(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteListText(&buf, logparse.Entry{Raw: "raw line"}); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "raw line\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteListJSON(t *testing.T) {
	raw := `45.146.85.105 - reichert8708 [01/Oct/2026:18:56:02 -0600] "GET /24%2f7/morph?a=1&b=2 HTTP/1.1" 400 29820`
	e, err := logparse.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteListJSON(&buf, "testdata/test.log", 1, e); err != nil {
		t.Fatal(err)
	}
	want := `{"file":"testdata/test.log","line":1,"ip":"45.146.85.105","user":"reichert8708","time":"2026-10-01T18:56:02-06:00","method":"GET","path":"/24%2f7/morph?a=1&b=2","protocol":"HTTP/1.1","status":400,"bytes":29820,"referer":"","agent":"","raw":"45.146.85.105 - reichert8708 [01/Oct/2026:18:56:02 -0600] \"GET /24%2f7/morph?a=1&b=2 HTTP/1.1\" 400 29820"}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRequestsPerMinute(t *testing.T) {
	tests := []struct {
		name    string
		entries []logparse.Entry
		want    float64
	}{
		{"empty", nil, 0},
		{"single entry", []logparse.Entry{entry(200, t0)}, 1},
		{"same timestamp", []logparse.Entry{entry(200, t0), entry(200, t0), entry(200, t0)}, 3},
		{"span under a minute counts as one", []logparse.Entry{entry(200, t0), entry(200, t0.Add(59*time.Second))}, 2},
		{"ten minutes", []logparse.Entry{entry(200, t0), entry(200, t0.Add(5*time.Minute)), entry(200, t0.Add(10*time.Minute))}, 0.3},
		{"rounded to two decimals", []logparse.Entry{entry(200, t0), entry(200, t0.Add(3*time.Minute))}, 0.67},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSummary()
			for _, e := range tc.entries {
				s.Add(e)
			}
			if got := s.RequestsPerMinute(); got != tc.want {
				t.Errorf("RequestsPerMinute() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSummaryText(t *testing.T) {
	tests := []struct {
		name    string
		entries []logparse.Entry
		want    string
	}{
		{
			name: "empty",
			want: "Total requests:      0\n" +
				"Requests per minute: 0.00\n",
		},
		{
			name: "range keeps original offsets and classes are sorted",
			entries: []logparse.Entry{
				entry(503, t0.Add(2*time.Minute)),
				entry(200, t0),
				entry(404, t0.Add(4*time.Minute).In(time.UTC)), // latest, in UTC
				entry(201, t0.Add(time.Minute)),
			},
			want: "Total requests:      4\n" +
				"Time range:          2026-10-01T18:00:00-06:00 – 2026-10-02T00:04:00Z\n" +
				"Requests per minute: 1.00\n" +
				"\n" +
				"Status codes:\n" +
				"  2xx      2  50.0%\n" +
				"    200    1  25.0%\n" +
				"    201    1  25.0%\n" +
				"  4xx      1  25.0%\n" +
				"    404    1  25.0%\n" +
				"  5xx      1  25.0%\n" +
				"    503    1  25.0%\n",
		},
		{
			name:    "one third percentages",
			entries: []logparse.Entry{entry(200, t0), entry(200, t0), entry(500, t0)},
			want: "Total requests:      3\n" +
				"Time range:          2026-10-01T18:00:00-06:00 – 2026-10-01T18:00:00-06:00\n" +
				"Requests per minute: 3.00\n" +
				"\n" +
				"Status codes:\n" +
				"  2xx      2  66.7%\n" +
				"    200    2  66.7%\n" +
				"  5xx      1  33.3%\n" +
				"    500    1  33.3%\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSummary()
			for _, e := range tc.entries {
				s.Add(e)
			}
			var buf bytes.Buffer
			if err := s.WriteText(&buf); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestSummaryJSON(t *testing.T) {
	tests := []struct {
		name    string
		entries []logparse.Entry
		want    string
	}{
		{
			name: "empty",
			want: `{
  "total": 0,
  "time_range": null,
  "requests_per_minute": 0,
  "status": {}
}
`,
		},
		{
			name:    "two classes",
			entries: []logparse.Entry{entry(500, t0), entry(200, t0.Add(2*time.Minute)), entry(200, t0)},
			want: `{
  "total": 3,
  "time_range": {
    "start": "2026-10-01T18:00:00-06:00",
    "end": "2026-10-01T18:02:00-06:00"
  },
  "requests_per_minute": 1.5,
  "status": {
    "2xx": {
      "count": 2,
      "percent": 66.7,
      "codes": {
        "200": 2
      }
    },
    "5xx": {
      "count": 1,
      "percent": 33.3,
      "codes": {
        "500": 1
      }
    }
  }
}
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSummary()
			for _, e := range tc.entries {
				s.Add(e)
			}
			var buf bytes.Buffer
			if err := s.WriteJSON(&buf); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
