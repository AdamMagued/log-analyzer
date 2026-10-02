# Filter language

The `-f` / `--filter` flag takes a boolean expression. Each log entry is
checked against it, and only entries for which it is true are listed or
summarized. Without `-f`, every entry matches.

```sh
log-analyzer -m list -f 'method == "POST" && status >= 400' access.log
```

Wrap the expression in single quotes so that the shell doesn't interpret
`"`, `!`, `&`, `|`, `<` or `>`.

## Comparisons

Every expression is built from comparisons with this shape:

```
field operator value
```

The field is always on the left and a literal value always on the right.
`status == 500` is valid; `500 == status` and `status == bytes` are not.

## Fields

| Field | Type | Description |
| --- | --- | --- |
| `ip` | string | Client address (`%h`) |
| `user` | string | Authenticated user (`%u`). `""` when the log has `-` |
| `time` | time | Request timestamp (`%t`) |
| `method` | string | HTTP method, e.g. `GET` |
| `path` | string | Request path as it appears in the log: not URL-decoded, query string included |
| `protocol` | string | e.g. `HTTP/1.1` |
| `status` | int | Response status code |
| `bytes` | int | Response size. `0` when the log has `-` |
| `referer` | string | Referer header. `""` for Common Log Format lines |
| `agent` | string | User agent. `""` for Common Log Format lines |

## Values

| Kind | Examples | Notes |
| --- | --- | --- |
| Integer | `200`, `1024` | Decimal digits only |
| String | `"GET"`, `"/users"` | Double quotes. `\"` is a quote, `\\` is a backslash. Any other backslash is kept as is, so `"\d+"` is the regex `\d+` |
| Time | `"2026-10-01T18:00:00-06:00"`, `"2026-10-02T00:00:00Z"` | A string in RFC 3339 format, used with the `time` field. The time zone offset is required |
| Status class | `1xx` … `5xx` | Only with `status`, and only with `==` or `!=` |

## Operators

| Operator | Meaning | Works with |
| --- | --- | --- |
| `==` | equal | every type |
| `!=` | not equal | every type |
| `<` `<=` `>` `>=` | ordering | `int` and `time` fields |
| `=~` | matches regex | `string` fields |
| `!~` | does not match regex | `string` fields |
| `!` | not | expressions |
| `&&` | and | expressions |
| `\|\|` | or | expressions |
| `( )` | grouping | expressions |

Details:

- **Strings:** comparisons are case-sensitive. Use the regex flag `(?i)` for
  case-insensitive matching, e.g. `agent =~ "(?i)bot"`.
- **Regexes:** use [Go RE2 syntax](https://pkg.go.dev/regexp/syntax) and match
  anywhere in the value unless anchored with `^` and `$`.
- **Times:** comparisons use absolute instants, so
  `"2026-10-01T18:00:00-06:00"` and `"2026-10-02T00:00:00Z"` are equal.

## Precedence

From highest to lowest:

1. `( … )`
2. `!`
3. Comparisons: `==` `!=` `<` `<=` `>` `>=` `=~` `!~`
4. `&&`
5. `||`

So `a || b && c` means `a || (b && c)`, and `!a && b` means `(!a) && b`.

Comparisons can't be chained: `status == 200 == 300` is an error. `&&` and
`||` short-circuit.

## Examples

```sh
# All server errors
status == 5xx

# Failed GET requests
method == "GET" && status >= 500

# Writes to a single user resource
path =~ "^/users/[0-9]+$" && !(method == "GET" || method == "HEAD")

# One hour of traffic
time >= "2026-10-01T18:00:00-06:00" && time < "2026-10-01T19:00:00-06:00"

# Bots, in any capitalization
agent =~ "(?i)bot"

# Large responses outside the API
bytes > 100000 && path !~ "^/api/"
```

## Errors

An invalid filter is reported before any input is read, and the tool exits
with code `2`. The message names the problem and points at the column where
it was found:

```
$ log-analyzer -f 'stauts >= 500' access.log
error: invalid filter: unknown field "stauts" at column 1
  stauts >= 500
  ^
```

```
$ log-analyzer -f 'method > 3' access.log
error: invalid filter: operator ">" is not supported for string field "method" at column 8
  method > 3
         ^
```

Possible errors:

- **Syntax errors:** an unknown character such as a single `=`, `&` or `|`;
  an unterminated string; a missing value or `)`; or chained comparisons.
- **Unknown fields.**
- **Type mismatches:** e.g. `status == "500"`, `method > "GET"`, or
  `bytes == 5xx`.
- **Invalid regexes or time literals.**
