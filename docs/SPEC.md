# Log Analyzer Specs

## Summary

Log Analyzer is a CLI tool written in Go to provide insights on REST API log
entries. It is focused on parsing, filtering, and creating reports based on
.log files.

## Usage

```
log-analyzer [flags] FILE [FILE...]
```

Flags must come before the input files.

## Input

### Files

- One or more input files are given as positional arguments. At least one is
  required.
- Arguments may be glob patterns (e.g. `'logs/*.log'`). The tool expands them
  itself, so quoted patterns work too. A pattern that matches no files is an
  error.
- Files are processed in the order given; within a file, lines are processed
  in order.
- Only regular files are read. Reading from stdin is not supported.

### Log formats

Both Apache formats are supported and detected per line, so a file may mix
them.

**Common Log Format (CLF)**

```
%h %l %u %t "%r" %>s %b
45.146.85.105 - reichert8708 [01/Oct/2026:18:56:02 -0600] "GET /24%2f7/morph HTTP/1.1" 400 29820
```

**Combined Log Format**: CLF plus referer and user agent.

```
%h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-agent}i"
10.0.0.1 - - [01/Oct/2026:18:56:02 -0600] "GET /users HTTP/1.1" 200 512 "https://example.com/" "curl/8.5.0"
```

Parsing rules:

- `-` in the user or bytes field means "absent". A `-` for bytes is treated
  as `0`.
- The timestamp format is `02/Jan/2006:15:04:05 -0700` and the offset is kept.
- The request line `"%r"` is split into method, path and protocol. The path is
  kept exactly as it appears in the log (not URL-decoded, query string
  included).
- The status must be between `100` and `599`. Any other value makes the line
  malformed.
- Empty lines are ignored and are not counted as malformed.

### Malformed lines

A line that matches neither format is malformed.

- **Default:** skip the line and keep going. At the end, if any lines were
  skipped, print one warning to stderr:
  `warning: skipped 3 malformed lines`
- **`--strict`:** stop at the first malformed line, print
  `error: access.log:42: malformed log line`, and exit with code `1`.

## Flags

| Short | Long | Description | Values | Default |
| --- | --- | --- | --- | --- |
| `-m` | `--mode` | Mode to run the analyzer in | `summary`, `list` | `summary` |
| `-f` | `--filter` | Filter expression (see [Filter language](#filter-language)) | expression | none (all entries match) |
| `-o` | `--format` | Output format | `text`, `json` | `text` |
| `-s` | `--strict` | Fail on the first malformed line | boolean | `false` |
| `-h` | `--help` | Print usage and exit | | |

Short and long forms are interchangeable. An invalid flag value is a usage error.

## Modes

Modes determine the output of the analyzer. Both operate only on entries
matching the filter.

### list

Outputs every matching entry, in input order.

- `text`: the raw log line, byte-for-byte as it appeared in the input.
- `json`: [JSON Lines](https://jsonlines.org/), one object per entry:

```json
{"file":"testdata/test.log","line":1,"ip":"45.146.85.105","user":"reichert8708","time":"2026-10-01T18:56:02-06:00","method":"GET","path":"/24%2f7/morph","protocol":"HTTP/1.1","status":400,"bytes":29820,"referer":"","agent":"","raw":"45.146.85.105 - reichert8708 [01/Oct/2026:18:56:02 -0600] \"GET /24%2f7/morph HTTP/1.1\" 400 29820"}
```

### summary

A report with:

- **Total requests:** number of matching entries.
- **Time range:** earliest and latest timestamps among matching entries.
  Comparisons use absolute instants. Each timestamp is printed with its
  original offset.
- **Requests per minute:** `total / max(span_in_minutes, 1)`, where
  `span = latest - earliest`. Spans under one minute count as one minute.
  The value is rounded to two decimals.
- **Status code distribution:** count and percentage per class (`1xx`–`5xx`),
  and per individual code within each class. Classes are listed in ascending
  order, and so are the codes within a class. Classes with no entries are
  omitted. Percentages are relative to the total and rounded to one decimal.

If no entries match, the summary still prints, with total `0`, no time
range, and `0` requests per minute.

## Filter language

`-f` takes a boolean expression that is evaluated against each entry. Only
entries for which it evaluates to true are kept.

### Fields

| Field | Type | Notes |
| --- | --- | --- |
| `ip` | string | Remote host |
| `user` | string | `""` if `-` |
| `time` | time | Request timestamp |
| `method` | string | e.g. `GET` |
| `path` | string | Raw request path, including query string |
| `protocol` | string | e.g. `HTTP/1.1` |
| `status` | int | Response status code |
| `bytes` | int | Response size, `0` if `-` |
| `referer` | string | `""` for CLF lines |
| `agent` | string | `""` for CLF lines |

### Literals

- **int:** `200`, `1024`
- **string:** double-quoted, with `\"` and `\\` escapes: `"GET"`
- **time:** an RFC 3339 string compared against `time`:
  `time >= "2026-10-01T18:00:00-06:00"`
- **status class:** `1xx`–`5xx`, only valid as `status == 5xx` or
  `status != 5xx`

### Operators

From highest to lowest precedence:

| Operator | Meaning | Operand types |
| --- | --- | --- |
| `( … )` | Grouping | |
| `!` | Logical NOT | bool |
| `==` `!=` | Equality | any (same type on both sides) |
| `<` `<=` `>` `>=` | Ordering | int, time |
| `=~` `!~` | Regex match / no match (Go RE2 syntax) | string field, string pattern |
| `&&` | Logical AND | bool |
| `\|\|` | Logical OR | bool |

All comparison operators share one precedence level and are non-associative,
so `a == b == c` is a syntax error. `&&` and `||` short-circuit. String
comparisons are case-sensitive.

### Grammar

```
expr       = or ;
or         = and { "||" and } ;
and        = unary { "&&" unary } ;
unary      = "!" unary | primary ;
primary    = "(" expr ")" | comparison ;
comparison = field op value ;
op         = "==" | "!=" | "<" | "<=" | ">" | ">=" | "=~" | "!~" ;
value      = INT | STRING | STATUS_CLASS ;
```

The left side of a comparison is always a field and the right side always a
literal.

### Errors

These are usage errors (exit code `2`) and are reported before any input is
read:

- syntax errors
- unknown fields
- type mismatches (e.g. `method > 3`)
- invalid regexes or time literals

The message points at the offending column:

```
error: invalid filter: unknown field "stauts" at column 1
  stauts >= 500
  ^
```

### Examples

```
status == 5xx
method == "GET" && status >= 500
path =~ "^/users/[0-9]+$" && !(method == "GET" || method == "HEAD")
time >= "2026-10-01T18:00:00-06:00" && time < "2026-10-01T19:00:00-06:00"
agent =~ "(?i)bot"
```

## Output and errors

- Results go to **stdout**. Warnings and errors go to **stderr**, prefixed
  with `warning: ` or `error: `.
- `--format json` affects only stdout. Messages on stderr are always plain
  text.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success, including when no entries match |
| `1` | Runtime error: a file can't be opened or read, or there's a malformed line in `--strict` mode |
| `2` | Usage error: no input files, a glob with no matches, an invalid flag or flag value, or an invalid filter expression |

## Examples

These use `testdata/test.log` (1000 CLF lines, all with the same timestamp).

**Default summary**

```
$ log-analyzer testdata/test.log
Total requests:      1000
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
```

**Filtered list**

```
$ log-analyzer -m list -f 'method == "GET" && status == 5xx' testdata/test.log
221.232.91.44 - - [01/Oct/2026:18:56:02 -0600] "GET /open-source/metrics/facilitate/transition HTTP/1.0" 503 9791
215.200.205.199 - - [01/Oct/2026:18:56:02 -0600] "GET /niches HTTP/1.1" 502 27810
...
```

(38 lines in total.)

**JSON summary**

```
$ log-analyzer --format json -f 'status == 5xx' testdata/test.log
{
  "total": 238,
  "time_range": {"start": "2026-10-01T18:56:02-06:00", "end": "2026-10-01T18:56:02-06:00"},
  "requests_per_minute": 238,
  "status": {
    "5xx": {"count": 238, "percent": 100, "codes": {"500": 39, "501": 44, "502": 55, "503": 52, "504": 48}}
  }
}
```

When there are no matching entries, `time_range` is `null`.

## Non-goals

- Reading from stdin, compressed files, or following files as they grow
  (`tail -f`)
- Log formats other than Apache CLF and Combined
- Latency metrics (the supported formats have no response time)
- Path normalization, top-N reports, or per-IP statistics

## Testing

### Unit tests

- Every package has table-driven tests (`[]struct{ name string; ... }` run
  with `t.Run(tc.name, ...)`).
- The cases cover both valid and invalid input, at least:
  - **Log parsing:** CLF and Combined lines, `-` fields, malformed lines.
  - **Filter language:** the lexer, the parser (including precedence and
    syntax errors with their columns), type checking, and evaluation of every
    operator.
  - **Summary:** totals, time range, the requests-per-minute rule for spans
    under one minute, status percentages, and the empty-result case.

### End-to-end tests

- `main` is a thin wrapper around
  `run(args []string, stdout, stderr io.Writer) int`, which returns the exit
  code. The end-to-end tests call `run` directly, without building a binary.
- They check stdout, stderr and the exit code for every mode, format and flag,
  for `--strict`, for globs, and for each exit code in
  [Exit codes](#exit-codes).
- The [Examples](#examples) section is covered by end-to-end tests that
  assert the same output.
- Input fixtures live in `testdata/`.

### Definition of done

A change is done when `go test ./...` and `go vet ./...` both pass (`make
check`).

## Documentation

- **`README.md`** contains:
  - **Getting Started:** prerequisites (the Go version from `go.mod`), how to
    build and install, and a first command to run against
    `testdata/test.log`.
  - An overview of the functionality.
  - A flag table that matches the `--help` output exactly.
  - A couple of examples.
- **`docs/filter-language.md`** is the user reference for the filter language:
  fields, literals, operators, precedence, error messages and examples.
- **`docs/architecture.md`** describes the packages, their responsibilities,
  and how data flows from input files to output.
- This spec (`docs/SPEC.md`) is the source of truth. When behavior changes,
  update the spec first and the other documents in the same commit.

## Source Control

- Work happens directly on `main`.
- After each meaningful unit of work, create a git commit using
  [Conventional Commits](https://www.conventionalcommits.org/):
  `<type>(<scope>): <description>`.
  - **Types:** `feat`, `fix`, `test`, `docs`, `refactor`, `chore`
  - **Scopes:** `parser`, `filter`, `cli`, `summary`, `build`. The scope may be
    omitted for changes that span the whole project.
  - **Example:** `feat(filter): add regex operators`
- Don't commit unless the change meets the
  [Definition of done](#definition-of-done) (`make check` passes).
- Build output (the `log-analyzer` binary and `bin/`) is ignored by git.

## Style

Code should be human readable and concerns should be kept separate to help
with maintainability. Avoid overengineering. Concretely:

### Tooling

- Code is formatted with `gofmt` and is `go vet` clean.
- Use only the Go standard library, with no third-party modules.

### Layout

- A thin `main.go` at the repository root. It only calls `run` and passes the
  result to `os.Exit`.
- Packages live under `internal/`, one per concern:
  - `internal/logparse`: parsing log lines
  - `internal/filter`: the filter language (lexer → parser → evaluator)
  - `internal/report`: list and summary output in text and JSON
  - `internal/cli` (if needed): flag parsing and input file expansion

### Errors

- Functions return errors instead of panicking. Wrap errors with context
  using `fmt.Errorf("...: %w", err)`.
- Only `main` calls `os.Exit`. No other code calls `panic` or `log.Fatal`.

### State

- No global mutable state. Dependencies such as writers and options are
  passed explicitly.

### Flags

- Use the standard `flag` package and register each flag under both names
  (e.g. `-m` and `--mode` share one variable). Because `flag` stops at the
  first positional argument, flags must come before the files.

### Input

- Read input line by line (`bufio.Scanner` with a raised buffer limit) so
  memory use doesn't grow with file size. `summary` keeps only counters and
  min/max timestamps.

### Avoiding overengineering

- Don't add an interface until there are two implementations.
- Don't add config files, plugins, concurrency or extra abstraction layers.
