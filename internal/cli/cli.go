// Package cli implements the log-analyzer command: flag parsing, input file
// expansion, the read loop, and exit codes.
package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"log-analyzer/internal/filter"
	"log-analyzer/internal/logparse"
	"log-analyzer/internal/report"
)

// Exit codes, see docs/SPEC.md.
const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

// Usage is the text printed by --help. README.md contains the same text.
const Usage = `Usage: log-analyzer [flags] FILE [FILE...]

Analyze Apache (CLF or Combined) access logs. Flags must come before the files.
FILE may be a glob pattern such as 'logs/*.log'.

Flags:
  -m, --mode string     mode to run the analyzer in: summary, list (default "summary")
  -f, --filter string   filter expression, e.g. 'method == "GET" && status == 5xx'
  -o, --format string   output format: text, json (default "text")
  -s, --strict          fail on the first malformed line
  -h, --help            print usage and exit
`

// maxLineLength is the longest log line the scanner accepts.
const maxLineLength = 1024 * 1024

type options struct {
	mode   string
	format string
	filter *filter.Filter // nil matches every entry
	strict bool
	files  []string
}

// Run executes the command with args (without the program name) and returns
// the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(stdout, Usage)
		return exitOK
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		var ferr *filter.Error
		if errors.As(err, &ferr) {
			fmt.Fprintln(stderr, ferr.Caret())
		} else {
			fmt.Fprint(stderr, "\n"+Usage)
		}
		return exitUsage
	}

	for _, f := range opts.files {
		if err := checkRegularFile(f); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return exitRuntime
		}
	}

	out := bufio.NewWriter(stdout)
	skipped, err := analyze(opts, out)
	if flushErr := out.Flush(); err == nil && flushErr != nil {
		err = fmt.Errorf("write output: %w", flushErr)
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return exitRuntime
	}
	if skipped > 0 {
		fmt.Fprintf(stderr, "warning: skipped %d malformed %s\n", skipped, plural(skipped, "line", "lines"))
	}
	return exitOK
}

// parseArgs parses flags, validates their values, compiles the filter and
// expands the input files. Every error it returns is a usage error;
// flag.ErrHelp means help was requested.
func parseArgs(args []string) (options, error) {
	var (
		opts       options
		filterExpr string
	)

	fs := flag.NewFlagSet("log-analyzer", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Run prints errors and usage itself
	for _, name := range []string{"m", "mode"} {
		fs.StringVar(&opts.mode, name, "summary", "")
	}
	for _, name := range []string{"f", "filter"} {
		fs.StringVar(&filterExpr, name, "", "")
	}
	for _, name := range []string{"o", "format"} {
		fs.StringVar(&opts.format, name, "text", "")
	}
	for _, name := range []string{"s", "strict"} {
		fs.BoolVar(&opts.strict, name, false, "")
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	if opts.mode != "summary" && opts.mode != "list" {
		return options{}, fmt.Errorf("invalid mode %q: must be summary or list", opts.mode)
	}
	if opts.format != "text" && opts.format != "json" {
		return options{}, fmt.Errorf("invalid format %q: must be text or json", opts.format)
	}

	if filterExpr != "" {
		f, err := filter.Compile(filterExpr)
		if err != nil {
			return options{}, fmt.Errorf("invalid filter: %w", err)
		}
		opts.filter = f
	}

	if fs.NArg() == 0 {
		return options{}, errors.New("no input files")
	}
	files, err := expandInputs(fs.Args())
	if err != nil {
		return options{}, err
	}
	opts.files = files

	return opts, nil
}

// expandInputs expands glob patterns. Arguments without glob metacharacters
// are returned as-is so that missing files are reported when opened.
func expandInputs(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		if !strings.ContainsAny(arg, "*?[") {
			files = append(files, arg)
			continue
		}
		matches, err := filepath.Glob(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", arg, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no files match %q", arg)
		}
		files = append(files, matches...)
	}
	return files, nil
}

func checkRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file", path)
	}
	return nil
}

// analyze reads every input file, applies the filter and writes the output
// for the selected mode. It returns the number of skipped malformed lines.
func analyze(opts options, out io.Writer) (int, error) {
	var (
		skipped int
		summary = report.NewSummary()
	)

	emit := func(file string, line int, e logparse.Entry) error {
		if opts.filter != nil && !opts.filter.Match(e) {
			return nil
		}
		switch {
		case opts.mode == "summary":
			summary.Add(e)
			return nil
		case opts.format == "json":
			return report.WriteListJSON(out, file, line, e)
		default:
			return report.WriteListText(out, e)
		}
	}

	for _, file := range opts.files {
		n, err := readFile(file, opts.strict, emit)
		skipped += n
		if err != nil {
			return skipped, err
		}
	}

	if opts.mode == "summary" {
		if opts.format == "json" {
			return skipped, summary.WriteJSON(out)
		}
		return skipped, summary.WriteText(out)
	}
	return skipped, nil
}

// readFile parses path line by line and calls emit for every valid entry.
// Malformed lines are skipped and counted, or are an error when strict is
// set. Empty lines are ignored.
func readFile(path string, strict bool, emit func(file string, line int, e logparse.Entry) error) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	skipped := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), maxLineLength)
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := strings.TrimSuffix(sc.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		e, err := logparse.Parse(line)
		if err != nil {
			if strict {
				return skipped, fmt.Errorf("%s:%d: malformed log line", path, lineNo)
			}
			skipped++
			continue
		}
		if err := emit(path, lineNo, e); err != nil {
			return skipped, fmt.Errorf("write output: %w", err)
		}
	}
	if err := sc.Err(); err != nil {
		return skipped, fmt.Errorf("read %s: %w", path, err)
	}
	return skipped, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
