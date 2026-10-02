# log-analyzer

This is a log analyzer developed with Claude Code and Go as an AI coding
demonstration.

`log-analyzer` is a command-line tool that reads Apache access logs from REST
APIs, filters them with a small expression language, and either lists the
matching entries or prints a summary report.

## Getting Started

### Prerequisites

- Go 1.27.1 or later (see `go.mod`)
- `make` (optional, for the shortcuts below)

### Build and install

```sh
make build          # compiles to bin/log-analyzer
go install .        # or install log-analyzer into $(go env GOPATH)/bin
```

### First run

The repository includes a sample log with 1000 entries:

```sh
bin/log-analyzer testdata/test.log
```

```
Total requests:      1000
Time range:          2026-10-01T18:56:02-06:00 – 2026-10-01T18:56:02-06:00
Requests per minute: 1000.00

Status codes:
  1xx     49   4.9%
    100   49   4.9%
  2xx    248  24.8%
...
```

## Overview

- **Input:** one or more log files or glob patterns, in Apache
  [Common Log Format](https://httpd.apache.org/docs/current/logs.html#common)
  or Combined Log Format. Both formats are detected line by line.
- **Filtering:** `-f` takes an expression such as
  `method == "GET" && status == 5xx`. See
  [docs/filter-language.md](docs/filter-language.md) for details.
- **Modes:**
  - `summary` (the default) reports total requests, time range, requests per
    minute and status code distribution.
  - `list` prints the matching log lines.
- **Output:** human-readable text, or JSON with `-o json`.
- **Malformed lines:** skipped with a warning, or a hard error with `--strict`.
- **Exit codes:**
  - `0`: success
  - `1`: runtime error (unreadable file, or a malformed line with `--strict`)
  - `2`: usage error (bad flags, bad filter, or no input files)

## Flags

This is the output of `log-analyzer --help`:

```
Usage: log-analyzer [flags] FILE [FILE...]

Analyze Apache (CLF or Combined) access logs. Flags must come before the files.
FILE may be a glob pattern such as 'logs/*.log'.

Flags:
  -m, --mode string     mode to run the analyzer in: summary, list (default "summary")
  -f, --filter string   filter expression, e.g. 'method == "GET" && status == 5xx'
  -o, --format string   output format: text, json (default "text")
  -s, --strict          fail on the first malformed line
  -h, --help            print usage and exit
```

## Examples

List the `GET` requests that failed with a server error:

```sh
$ bin/log-analyzer -m list -f 'method == "GET" && status == 5xx' testdata/test.log
221.232.91.44 - - [01/Oct/2026:18:56:02 -0600] "GET /open-source/metrics/facilitate/transition HTTP/1.0" 503 9791
215.200.205.199 - - [01/Oct/2026:18:56:02 -0600] "GET /niches HTTP/1.1" 502 27810
...
```

Summarize the server errors as JSON:

```sh
$ bin/log-analyzer --format json -f 'status == 5xx' testdata/test.log
{
  "total": 238,
  ...
}
```

Export the 4xx responses of a time window from several files as JSON Lines:

```sh
bin/log-analyzer -m list -o json \
  -f 'status == 4xx && time >= "2026-10-01T18:00:00-06:00"' 'logs/*.log'
```

## Development

```sh
make check   # gofmt check, go vet and all tests (the definition of done)
make test    # tests only
make clean   # remove bin/
```

## Documentation

- [docs/SPEC.md](docs/SPEC.md): the specification and the source of truth
- [docs/filter-language.md](docs/filter-language.md): the filter language
  reference
- [docs/architecture.md](docs/architecture.md): packages and data flow
