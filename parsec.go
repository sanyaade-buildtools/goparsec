package parsec

import (
	"bytes"
	"fmt"
)

// parse combinator function
type Parser func (*ParseState) (interface{}, error)

// common parser combinators
var Lowercase = OneOf([]byte("abcdefghijklmnopqrstuvwxyz"))
var Uppercase = OneOf([]byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ"))
var Letter = Either(Lowercase, Uppercase)
var Letters = Many1(Letter)
var Digit = OneOf([]byte("0123456789"))
var Digits = Many1(Digit)
var AlphaNum = Either(Letter, Digit)
var AlphaNums = Many1(AlphaNum)
var HexDigit = OneOf([]byte("0123456789acdefABCDEF"))
var HexDigits = Many1(HexDigit)
var Punctuation = OneOf([]byte("!@#$%^&*()-=+[]{}\\|;:'\",./<>?~`"))
var Space = OneOf([]byte(" \t"))
var Spaces = Skip(Space)
var Newline = OneOf([]byte("\r\n"))
var Eol = Either(Eof, Newline)

// the current parser state
type ParseState struct {
	Source string
	Pos int
	Line int
}

// when parsing fails, this is why
type ParseErr struct {
	Reason string
	Line int
}

// get the parse error text for a given error
func (err ParseErr) Error() string {
	return fmt.Sprintf("%s on line %d", err.Reason, err.Line)
}

// entry point for parsing data
func Parse(source string, p Parser) (interface{}, error) {
	st := ParseState{
		Source: source,
		Line: 1,
		Pos: 0,
	}

	// call the parse function with the state
	return p(&st)
}

// get the next character in the parse stream
func (st *ParseState) next(pred func(byte) bool) (byte, bool) {
	if st.Pos < len(st.Source) {
		c := st.Source[st.Pos]

		// make sure the predicate matches
		if pred(c) == false {
			return c, false
		}

		// advance the stream position
		st.Pos++

		// advance to another line?
		if c == '\n' {
			st.Line++
		}

		return c, true
	}

	// just a null character and failure
	return '\000', false
}

// generate a formated parse error
func (st *ParseState) trap(format string, args... interface{}) ParseErr {
	return ParseErr{Line: st.Line, Reason: fmt.Sprintf(format, args...)}
}

// bind a parse combinator to a function, return a new combinator
func Bind(p Parser, f func(interface{}) Parser) Parser {
	return func(st *ParseState) (interface{}, error) {
		x, err := p(st)

		// check for an error
		if err != nil {
			return nil, err
		}

		return f(x)(st)
	}
}

// bind two parse combinators, ignore the intermediate result
func Bind_(p1, p2 Parser) Parser {
	return func(st *ParseState) (interface{}, error) {
		_, err := p1(st)

		// check for an error
		if err != nil {
			return nil, err
		}

		return p2(st)
	}
}

// drop a value down into the parse monad
func Return(x interface{}) Parser {
	return func(st *ParseState) (interface{}, error) {
		return x, nil
	}
}

// throw an error
func Fail(msg string) Parser {
	return func(st *ParseState) (interface{}, error) {
		return nil, st.trap(msg)
	}
}

// try one parser, if it fails (without consuming input) try the next
func Either(p1, p2 Parser) Parser {
	return func(st *ParseState) (interface{}, error) {
		oldPos := st.Pos

		// try the first parser
		x, err := p1(st)

		// success?
		if err == nil {
			return x, nil
		}

		// make sure no input was consumed
		if st.Pos == oldPos {
			return p2(st)
		}

		return nil, err
	}
}

// attempt to match a parser, if it fails pretend no input was consumed
func Try(p Parser) Parser {
	return func(st *ParseState) (interface{}, error) {
		oldPos := st.Pos

		// try the first parser
		x, err := p(st)

		// on success, return the value
		if err == nil {
			return x, nil
		}

		// reset back to the original position
		st.Pos = oldPos

		// this result value should be ignored
		return nil, err
	}
}

// accept any valid character
func AnyChar(st *ParseState) (interface{}, error) {
	c, ok := st.next(func(x byte) bool { return true })

	if ok {
		return c, nil
	}

	return nil, st.trap("Unexpected end of file")
}

// check for the end of the file
func Eof(st *ParseState) (interface{}, error) {
	c, ok := st.next(func(x byte) bool { return true })

	if ok {
		return nil, st.trap("Expected end of file but got '%c'", c)
	}

	return nil, nil
}

// check for the next character being a specific one
func Char(c byte) Parser {
	return func(st *ParseState) (interface{}, error) {
		x, ok := st.next(func(b byte) bool { return b == c })

		if ok {
			return x, nil
		}

		return nil, st.trap("Expected '%c'", c)
	}
}

// check for the next character being from a set
func OneOf(set []byte) Parser {
	return func(st *ParseState) (interface{}, error) {
		x, ok := st.next(func(c byte) bool { return bytes.IndexByte(set, c) >= 0 })

		if ok {
			return x, nil
		}

		return nil, st.trap("Expected one of '%s' but got '%c'", string(set), x)
	}
}

// check for the next character not being from a set
func NoneOf(set []byte) Parser {
	return func(st *ParseState) (interface{}, error) {
		x, ok := st.next(func(c byte) bool { return bytes.IndexByte(set, c) < 0 })

		if ok {
			return x, nil
		}

		return nil, st.trap("Unexpected '%c'", x)
	}
}

// match an exact string, don't consume input if a match fails
func String(s string) Parser {
	return func(st *ParseState) (interface{}, error) {
		oldPos := st.Pos

		// try and match each character
		for _, c := range []byte(s) {
			_, ok := st.next(func(b byte) bool { return b == c })

			if ok == false {
				st.Pos = oldPos

				// the string failed to match
				return nil, st.trap("Expected '%s'", s)
			}
		}

		return s, nil
	}
}

// optionally parse a combinator or return a default value
func Option(x interface{}, p Parser) Parser {
	return Either(p, Return(x))
}

// match a parse combinator one or more times
func Many1(p Parser) Parser {
	head := func(x interface{}) Parser {
		tail := func(xs interface{}) Parser {
			return Return(append([]interface{}{x}, xs.([]interface{})...))
		}

		return Bind(Many(p), tail)
	}

	return Bind(p, head)
}

// match a parse combinator zero or more times
func Many(p Parser) Parser {
	return Option([]interface{}{}, Many1(p))
}

// ignore a combinator if present
func Maybe(p Parser) Parser {
	return Option(nil, Bind_(p, Return(nil)))
}

// skip a combinator zero or more times
func Skip(p Parser) Parser {
	return Maybe(Many(p))
}

// capture what's between two parsers
func Between(start, end, p Parser) Parser {
	keep := func(x interface{}) Parser {
		return Bind_(end, Return(x))
	}

	return Bind_(start, Bind(p, keep))
}

// captures a combinator separated one or more times by another
func SepBy1(p, sep Parser) Parser {
	head := func(x interface{}) Parser {
		tail := func(xs interface{}) Parser {
			return Return(append([]interface{}{x}, xs.([]interface{})...))
		}

		return Bind(Many(Bind_(sep, p)), tail)
	}

	return Bind(p, head)
}

// captures a combinator separated zero or more times by another
func SepBy(p, sep Parser) Parser {
	return Option([]interface{}{}, SepBy1(p, sep))
}

// keep capturing a parser until a terminal is found
func ManyTil(p, end Parser) Parser {
	head := func(x interface{}) Parser {
		tail := func(xs interface{}) Parser {
			return Return(append([]interface{}{x}, xs.([]interface{})...))
		}

		return Bind(ManyTil(p, end), tail)
	}

	// the terminal parser
	term := Bind_(Try(end), Return([]interface{}{}))

	// keep trying to parse end, if it fails, parse p instead
	return Either(term, Bind(p, head))
}
