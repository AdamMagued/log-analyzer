# Architecture

`log-analyzer` is a single Go module with no third-party dependencies. The
code is split into one package per concern, all under `internal/`.

```
main.go               os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
internal/
  cli/                flags, input files, read loop, exit codes
  logparse/           one log line → logparse.Entry
  filter/             filter expression → *filter.Filter (lexer, parser, evaluator)
  report/             list output and the summary, as text or JSON
testdata/             sample logs used by the tests
```

Dependencies only point downward: `cli` uses the other three packages,
`filter` and `report` use `logparse`, and `logparse` uses only the standard
library.

## Data flow

```
args ──► cli.parseArgs ──► options{mode, format, filter, strict, files}
                │
                ├─ flag parsing and validation      ─┐
                ├─ filter.Compile(expr)              ├─ any error: exit 2
                └─ glob expansion                   ─┘
                │
          checkRegularFile(each file)                ── error: exit 1
                │
for each file, for each line ──► logparse.Parse ──► malformed? skip and count,
                                       │             or exit 1 with --strict
                                       ▼
                              filter.Match(entry)?
                                       │
                     ┌─────────────────┴─────────────────┐
                  list mode                         summary mode
     report.WriteListText / WriteListJSON        report.Summary.Add
          (written immediately)                         │
                                                 after the last file:
                                          Summary.WriteText / WriteJSON
                │
     warning: skipped N malformed lines (stderr)
```

Everything is checked before any input is read: flags, the filter and the
glob patterns. That way usage errors never produce partial output.

## Packages

### `internal/logparse`

`Parse(line) (Entry, error)` matches a line against one regular expression.
That expression handles both formats, because Combined is CLF plus an
optional `"referer" "agent"` suffix. It then validates the timestamp, the
three-part request line, and the status range (100–599). Every failure wraps
`ErrMalformed`. `Entry.Raw` keeps the original line so that list mode can
print it byte-for-byte.

### `internal/filter`

A classic three-stage interpreter:

1. **Lexer** (`lexer.go`): turns the expression into tokens, each with its
   1-based column. String escapes and status classes (`5xx`) are resolved here.
2. **Parser** (`parser.go`): a recursive-descent parser with one function per
   grammar rule (`or` → `and` → `unary` → `primary` → `comparison`). Because
   of the rule order, `&&` binds tighter than `||`.
3. **Type checking and evaluation** (`filter.go`): `compare` looks up the field
   in the `fields` table and checks the operator and value types. It also
   compiles regexes and time literals once, then returns a typed node
   (`stringNode`, `intNode`, `regexNode`, `timeNode`, `classNode`). Evaluation
   walks the tree: `Filter.Match(entry)`.

Every error is a `*filter.Error` with the expression, column and message.
`Caret()` renders the line with a `^` that the CLI prints.

### `internal/report`

- **List output:** `WriteListText` prints `Entry.Raw`, and `WriteListJSON`
  prints one JSON Lines object.
- **`Summary`:** an accumulator that keeps only the total, the earliest and
  latest timestamps, and a count per status code. Memory use therefore does
  not grow with the size of the input. `WriteText` and `WriteJSON` group the
  counts by class when rendering.

### `internal/cli`

`Run(args, stdout, stderr) int` is the whole program. `main` only passes it
the real streams and exits with the returned code. Because of this the
end-to-end tests can call `Run` with buffers, without building a binary.

- **Files:** read with a `bufio.Scanner` whose line limit is raised to 1 MiB.
- **Output:** stdout is buffered.
- **Messages:** errors and warnings go to stderr with an `error:` or
  `warning:` prefix.

## Testing

- **Unit tests:** every package has table-driven tests.
- **End-to-end tests:** `internal/cli/cli_test.go` runs `Run` from the
  repository root against the files in `testdata/`. It checks stdout, stderr
  and the exit code, including the exact examples from `docs/SPEC.md`. It
  also checks that `README.md` contains the `--help` text.
- **Running them:** `make check` runs the gofmt check, `go vet`, and all tests.
