package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot is resolved before any test changes the working directory.
var repoRoot, _ = filepath.Abs("../..")

// run calls Run from the repository root, so paths match docs/SPEC.md.
func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	t.Chdir(repoRoot)
	var out, errOut bytes.Buffer
	code = Run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The three tests below cover the Examples section of docs/SPEC.md.

func TestSpecDefaultSummary(t *testing.T) {
	want := `Total requests:      1000
Time range:          2026-10-01T18:56:02-06:00 – 2026-10-01T18:56:02-06:00
Requests per minute: 1000.00

Status codes:
  1xx     49   4.9%
    100   49   4.9%
  2xx    248  24.8%
    200   53   5.3%
    201   48   4.8%
    203   47   4.7%
    204   40   4.0%
    205   60   6.0%
  3xx    139  13.9%
    301   51   5.1%
    302   37   3.7%
    304   51   5.1%
  4xx    326  32.6%
    400   55   5.5%
    401   43   4.3%
    403   43   4.3%
    404   38   3.8%
    405   55   5.5%
    406   40   4.0%
    416   52   5.2%
  5xx    238  23.8%
    500   39   3.9%
    501   44   4.4%
    502   55   5.5%
    503   52   5.2%
    504   48   4.8%
`
	stdout, stderr, code := run(t, "testdata/test.log")
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestSpecFilteredList(t *testing.T) {
	stdout, stderr, code := run(t, "-m", "list", "-f", `method == "GET" && status == 5xx`, "testdata/test.log")
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 38 {
		t.Errorf("got %d lines, want 38", len(lines))
	}
	wantFirst := []string{
		`221.232.91.44 - - [01/Oct/2026:18:56:02 -0600] "GET /open-source/metrics/facilitate/transition HTTP/1.0" 503 9791`,
		`215.200.205.199 - - [01/Oct/2026:18:56:02 -0600] "GET /niches HTTP/1.1" 502 27810`,
	}
	for i, want := range wantFirst {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i+1, lines[i], want)
		}
	}
}

func TestSpecJSONSummary(t *testing.T) {
	want := `{
  "total": 238,
  "time_range": {
    "start": "2026-10-01T18:56:02-06:00",
    "end": "2026-10-01T18:56:02-06:00"
  },
  "requests_per_minute": 238,
  "status": {
    "5xx": {
      "count": 238,
      "percent": 100,
      "codes": {
        "500": 39,
        "501": 44,
        "502": 55,
        "503": 52,
        "504": 48
      }
    }
  }
}
`
	stdout, stderr, code := run(t, "--format", "json", "-f", "status == 5xx", "testdata/test.log")
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestRun(t *testing.T) {
	combined := `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200 512 "https://example.com/" "curl/8.5.0"` + "\n"
	globA1 := `10.0.1.1 - - [01/Oct/2026:18:56:02 -0600] "GET /from-a HTTP/1.1" 200 1` + "\n"
	globA2 := `10.0.1.2 - - [01/Oct/2026:18:56:03 -0600] "GET /from-a HTTP/1.1" 500 1` + "\n"
	globB1 := `10.0.2.1 - - [01/Oct/2026:18:56:04 -0600] "GET /from-b HTTP/1.1" 200 1` + "\n"
	malformed1 := `10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200 512` + "\n"

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // compared exactly unless stdoutHas is set
		stdoutHas  []string
		wantStderr string // compared exactly unless stderrHas is set
		stderrHas  []string
	}{
		{
			name:       "list text with Combined format and filter on agent",
			args:       []string{"-m", "list", "-f", `agent == "curl/8.5.0"`, "testdata/combined.log"},
			wantStdout: combined,
		},
		{
			name:       "list json",
			args:       []string{"-m", "list", "-o", "json", "-f", `user == "alice"`, "testdata/combined.log"},
			wantStdout: `{"file":"testdata/combined.log","line":2,"ip":"10.0.0.2","user":"alice","time":"2026-10-01T18:56:03-06:00","method":"POST","path":"/users","protocol":"HTTP/1.1","status":201,"bytes":128,"referer":"-","agent":"Mozilla/5.0 (X11; Linux x86_64)","raw":"10.0.0.2 - alice [01/Oct/2026:18:56:03 -0600] \"POST /users HTTP/1.1\" 201 128 \"-\" \"Mozilla/5.0 (X11; Linux x86_64)\""}` + "\n",
		},
		{
			name: "summary across offsets and minutes",
			args: []string{"testdata/span.log"},
			stdoutHas: []string{
				"Total requests:      6\n",
				"Time range:          2026-10-01T18:00:00-06:00 – 2026-10-02T00:10:00Z\n",
				"Requests per minute: 0.60\n",
			},
		},
		{
			name: "summary with no matches",
			args: []string{"-f", "status == 1xx", "testdata/combined.log"},
			wantStdout: "Total requests:      0\n" +
				"Requests per minute: 0.00\n",
		},
		{
			name:       "json summary with no matches",
			args:       []string{"-o", "json", "-f", "status == 1xx", "testdata/combined.log"},
			wantStdout: "{\n  \"total\": 0,\n  \"time_range\": null,\n  \"requests_per_minute\": 0,\n  \"status\": {}\n}\n",
		},
		{
			name: "time filter",
			args: []string{"-m", "list", "-f", `time >= "2026-10-01T18:05:00-06:00" && time < "2026-10-01T18:08:00-06:00"`, "testdata/span.log"},
			wantStdout: `10.0.0.3 - - [02/Oct/2026:00:05:00 +0000] "GET /c HTTP/1.1" 301 1` + "\n" +
				`10.0.0.4 - - [01/Oct/2026:18:07:30 -0600] "GET /d HTTP/1.1" 404 1` + "\n",
		},
		{
			name:       "malformed lines are skipped with a warning",
			args:       []string{"-m", "list", "testdata/malformed.log"},
			stdoutHas:  []string{malformed1},
			wantStderr: "warning: skipped 3 malformed lines\n",
		},
		{
			name:       "strict stops at the first malformed line",
			args:       []string{"--strict", "testdata/malformed.log"},
			wantCode:   1,
			wantStderr: "error: testdata/malformed.log:2: malformed log line\n",
		},
		{
			name:       "strict list keeps lines printed before the error",
			args:       []string{"-s", "-m", "list", "testdata/malformed.log"},
			wantCode:   1,
			wantStdout: malformed1,
			wantStderr: "error: testdata/malformed.log:2: malformed log line\n",
		},
		{
			name:       "quoted glob is expanded in sorted order",
			args:       []string{"-m", "list", "testdata/glob/*.log"},
			wantStdout: globA1 + globA2 + globB1,
		},
		{
			name:       "files are read in the order given",
			args:       []string{"-m", "list", "testdata/glob/b.log", "testdata/glob/a.log"},
			wantStdout: globB1 + globA1 + globA2,
		},
		{
			name:       "help",
			args:       []string{"--help"},
			wantStdout: Usage,
		},
		{
			name:       "short help",
			args:       []string{"-h"},
			wantStdout: Usage,
		},
		{
			name:      "no input files",
			args:      []string{"-m", "list"},
			wantCode:  2,
			stderrHas: []string{"error: no input files\n", "Usage:"},
		},
		{
			name:      "glob without matches",
			args:      []string{"testdata/nothing-*.log"},
			wantCode:  2,
			stderrHas: []string{`error: no files match "testdata/nothing-*.log"`},
		},
		{
			name:      "unknown flag",
			args:      []string{"--verbose", "testdata/test.log"},
			wantCode:  2,
			stderrHas: []string{"error: flag provided but not defined: -verbose\n", "Usage:"},
		},
		{
			name:      "invalid mode",
			args:      []string{"-m", "report", "testdata/test.log"},
			wantCode:  2,
			stderrHas: []string{`error: invalid mode "report": must be summary or list`},
		},
		{
			name:      "invalid format",
			args:      []string{"--format", "xml", "testdata/test.log"},
			wantCode:  2,
			stderrHas: []string{`error: invalid format "xml": must be text or json`},
		},
		{
			name:       "invalid filter is reported with a caret before reading input",
			args:       []string{"-f", `method > 3`, "testdata/missing.log"},
			wantCode:   2,
			wantStderr: "error: invalid filter: operator \">\" is not supported for string field \"method\" at column 8\n  method > 3\n         ^\n",
		},
		{
			name:      "missing file",
			args:      []string{"testdata/missing.log"},
			wantCode:  1,
			stderrHas: []string{"error: stat testdata/missing.log: no such file or directory"},
		},
		{
			name:       "directory is not a regular file",
			args:       []string{"testdata/glob"},
			wantCode:   1,
			wantStderr: "error: testdata/glob: not a regular file\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := run(t, tc.args...)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d (stderr %q)", code, tc.wantCode, stderr)
			}
			checkOutput(t, "stdout", stdout, tc.wantStdout, tc.stdoutHas)
			checkOutput(t, "stderr", stderr, tc.wantStderr, tc.stderrHas)
		})
	}
}

func checkOutput(t *testing.T, name, got, want string, has []string) {
	t.Helper()
	if has == nil {
		if got != want {
			t.Errorf("%s =\n%s\nwant\n%s", name, got, want)
		}
		return
	}
	for _, s := range has {
		if !strings.Contains(got, s) {
			t.Errorf("%s =\n%s\nwant it to contain %q", name, got, s)
		}
	}
}

func TestListPrintsWholeFileWithoutFilter(t *testing.T) {
	stdout, stderr, code := run(t, "--mode", "list", "testdata/combined.log")
	if code != 0 || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if want := readFixture(t, "testdata/combined.log"); stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestShortAndLongFlagsAreEquivalent(t *testing.T) {
	filter := `status >= 400 && method != "HEAD"`
	shortOut, _, shortCode := run(t, "-m", "list", "-o", "json", "-f", filter, "testdata/test.log")
	longOut, _, longCode := run(t, "--mode", "list", "--format", "json", "--filter", filter, "testdata/test.log")
	if shortCode != 0 || longCode != 0 {
		t.Fatalf("exit codes = %d, %d", shortCode, longCode)
	}
	if shortOut == "" || shortOut != longOut {
		t.Errorf("short and long flags produced different output")
	}
}

func TestHelpMatchesReadme(t *testing.T) {
	readme := readFixture(t, filepath.Join(repoRoot, "README.md"))
	if !strings.Contains(readme, Usage) {
		t.Errorf("README.md does not contain the --help output:\n%s", Usage)
	}
}
