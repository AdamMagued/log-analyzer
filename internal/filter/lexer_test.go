package filter

import (
	"errors"
	"reflect"
	"testing"
)

func TestLex(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want []token
	}{
		{
			name: "empty",
			expr: "",
			want: []token{{tokEOF, "", 1}},
		},
		{
			name: "comparison with int",
			expr: "status >= 500",
			want: []token{{tokIdent, "status", 1}, {tokGe, "", 8}, {tokInt, "500", 11}, {tokEOF, "", 14}},
		},
		{
			name: "status class",
			expr: "status==5xx",
			want: []token{{tokIdent, "status", 1}, {tokEq, "", 7}, {tokClass, "5", 9}, {tokEOF, "", 12}},
		},
		{
			name: "string with escapes",
			expr: `agent == "a \"b\" \\ \d"`,
			want: []token{{tokIdent, "agent", 1}, {tokEq, "", 7}, {tokString, `a "b" \ \d`, 10}, {tokEOF, "", 25}},
		},
		{
			name: "all operators",
			expr: "( ) ! && || == != < <= > >= =~ !~",
			want: []token{
				{tokLParen, "", 1}, {tokRParen, "", 3}, {tokNot, "", 5}, {tokAnd, "", 7},
				{tokOr, "", 10}, {tokEq, "", 13}, {tokNe, "", 16}, {tokLt, "", 19},
				{tokLe, "", 21}, {tokGt, "", 24}, {tokGe, "", 26}, {tokMatch, "", 29},
				{tokNotMatch, "", 32}, {tokEOF, "", 34},
			},
		},
		{
			name: "not without space",
			expr: "!(a)",
			want: []token{{tokNot, "", 1}, {tokLParen, "", 2}, {tokIdent, "a", 3}, {tokRParen, "", 4}, {tokEOF, "", 5}},
		},
		{
			name: "columns count runes",
			expr: `path == "ñ" && x`,
			want: []token{{tokIdent, "path", 1}, {tokEq, "", 6}, {tokString, "ñ", 9}, {tokAnd, "", 13}, {tokIdent, "x", 16}, {tokEOF, "", 17}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lex(tc.expr)
			if err != nil {
				t.Fatalf("lex() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("lex() =\n  %v\nwant\n  %v", got, tc.want)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantCol int
		wantMsg string
	}{
		{"unterminated string", `path == "abc`, 9, "unterminated string"},
		{"unterminated after escape", `path == "abc\"`, 9, "unterminated string"},
		{"single equals", "status = 200", 8, `unexpected character '='`},
		{"single ampersand", "a & b", 3, `unexpected character '&'`},
		{"single pipe", "a | b", 3, `unexpected character '|'`},
		{"bad character", "status @ 1", 8, `unexpected character '@'`},
		{"number with letters", "status == 12ab", 11, `invalid number "12ab"`},
		{"bad status class", "status == 6xx", 11, `invalid number "6xx"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := lex(tc.expr)
			var ferr *Error
			if !errors.As(err, &ferr) {
				t.Fatalf("lex() error = %v, want *Error", err)
			}
			if ferr.Col != tc.wantCol || ferr.Msg != tc.wantMsg {
				t.Errorf("lex() error = {%d, %q}, want {%d, %q}", ferr.Col, ferr.Msg, tc.wantCol, tc.wantMsg)
			}
		})
	}
}

func TestErrorCaret(t *testing.T) {
	err := &Error{Col: 4, Msg: "boom"}
	want := "  a = b\n     ^"
	if got := err.Caret("a = b"); got != want {
		t.Errorf("Caret() =\n%s\nwant\n%s", got, want)
	}
	if got, want := err.Error(), "boom at column 4"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
