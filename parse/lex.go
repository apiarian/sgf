package parse

/*
This lexer was inspired by https://www.youtube.com/watch?v=HxaD_trXwRE and
adapted from
https://github.com/golang/go/blob/master/src/text/template/parse/lex.go

The lexer probably does more work than strictly necessary, especially the
lexProperty method. ¯\_(ツ)_/¯
*/

import "fmt"

type item struct {
	typ itemType
	pos Pos
	val []byte
}

type itemType int

const (
	itemError itemType = iota
	itemWarning
	itemOpenParen
	itemCloseParen
	itemOpenBracket
	itemCloseBracket
	itemSemiColon
	itemEOF
	itemPropertyIdent
	itemPropertyValue
)

const eof = 0

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError, i.typ == itemWarning:
		return string(i.val)
	case i.typ == itemOpenParen:
		return "("
	case i.typ == itemCloseParen:
		return ")"
	case i.typ == itemOpenBracket:
		return "["
	case i.typ == itemCloseBracket:
		return "]"
	case i.typ == itemSemiColon:
		return ";"
	case len(i.val) > 10:
		return fmt.Sprintf("%q...", i.val[:10])
	}
	return fmt.Sprintf("%q", i.val)
}

type stateFn func(*lexer) stateFn

type lexer struct {
	name      string
	input     []byte
	state     stateFn
	pos       Pos
	start     Pos
	width     Pos
	items     chan item
	treeDepth int
}

// next gets the next byte in the buffer or eof if there are no more bytes and
// advances the cursor
func (l *lexer) next() byte {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r := l.input[l.pos]
	l.width = 1
	l.pos += l.width
	return r
}

// peek gets the next byte in the buffer but does not advance the cursor
func (l *lexer) peek() byte {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one byte in the buffer
func (l *lexer) backup() {
	l.pos -= l.width
}

// emit sends an item on on the items channel and advances the start to the
// current position
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

// emit advances start to the current position without emitting the contents
// between start and pos to the items channel
func (l *lexer) ignore() {
	l.start = l.pos
}

// errorf emits a formatted error string as bytes on the items channel and
// halts the lexing process
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.start, []byte(fmt.Sprintf(format, args...))}
	return nil
}

// emitWarning emits a formatted warning string as bytes on the items channel
// and does not interrupt the lexing process
func (l *lexer) emitWarning(format string, args ...interface{}) {
	l.items <- item{itemWarning, l.start, []byte(fmt.Sprintf(format, args...))}
}

// lex starts the lexing process on a named slice of bytes
func lex(name string, input []byte) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l
}

// run process the lexer state until there is no more state to process
func (l *lexer) run() {
	for l.state = lexBytes; l.state != nil; {
		l.state = l.state(l)
	}
	close(l.items)
}

// lexBytes handles the outermost level of the bytes, ignoring all characters
// except the opening parentheses. The closing parentheses are also handled,
// but only to detect extra closing parentheses.
func lexBytes(l *lexer) stateFn {
Loop:
	for {
		n := l.next()
		switch n {
		case '(':
			return lexOpenParen
		case ')':
			return lexCloseParen
		case eof:
			break Loop
		default:
			l.ignore()
		}
	}
	l.emit(itemEOF)
	return nil
}

// lexOpenParen emits the open parantheses character and increments the treeDepth
func lexOpenParen(l *lexer) stateFn {
	l.emit(itemOpenParen)
	l.treeDepth++
	return lexTree
}

// lexCloseParen decrements the treeDepth and emits the closing parentheses
// charecter if it is sensible to do so
func lexCloseParen(l *lexer) stateFn {
	l.treeDepth--
	if l.treeDepth < 0 {
		return l.errorf("lex: too many right parentheses")
	}
	l.emit(itemCloseParen)
	if l.treeDepth > 0 {
		return lexTree
	} else {
		return lexBytes
	}
}

// lexTree deals with the upper levels of of a GameTree, handling semicolons,
// opening and closing parentheses. Other characters are ignored.
func lexTree(l *lexer) stateFn {
	for {
		n := l.next()
		switch n {
		case ';':
			return lexSemiColon
		case '(':
			return lexOpenParen
		case ')':
			return lexCloseParen
		case eof:
			return l.errorf("lex: unexpected EOF; expected GameTree contents or ')'")
		default:
			l.ignore()
		}
	}
}

// lexSemiColon emits a semicolon and changes context to deal with Properties
func lexSemiColon(l *lexer) stateFn {
	l.emit(itemSemiColon)
	return lexProperty
}

// consumeWhitespace ignores as much whitespace as it can find
func (l *lexer) consumeWhitespace() {
	for {
		n := l.next()
		if n == ' ' || n == '\n' || n == '\t' {
			l.ignore()
		} else {
			l.backup()
			break
		}
	}
}

// lexProperty handles the PropertyIdent and the PropertyValue parts of a
// property, emitting them and the opening and closing brackets that come with
// them
func lexProperty(l *lexer) stateFn {
	l.consumeWhitespace()

IdentLoop:
	for {
		n := l.next()
		switch {
		case n == eof:
			return l.errorf("lex: unexpected EOF; expected PropertyIdent")
		case n == ';':
			if semiColonWidth := l.pos - l.start; semiColonWidth == 1 {
				return lexSemiColon
			} else {
				return l.errorf("lex: unexpected ';'; expected PropertyIdent")
			}
		case n == '[':
			l.backup()
			break IdentLoop
		case n < 'A' || n > 'Z':
			return l.errorf("lex: PropertyIdent must be upper-case letters")
		}
	}
	if identWidth := l.pos - l.start; identWidth > 2 {
		l.emitWarning("lex: Found PropertyIdent wider than 2 characters")
	}
	l.emit(itemPropertyIdent)
	_ = l.next()
	l.emit(itemOpenBracket)

ValueLoop:
	for {
		n := l.next()
		switch {
		case n == eof:
			return l.errorf("lex: unexpected EOF; expected PropertyValue")
		case n == '\\':
			_ = l.next()
		case n == ']':
			l.backup()
			break ValueLoop
		}
	}
	l.emit(itemPropertyValue)
	_ = l.next()
	l.emit(itemCloseBracket)

	l.consumeWhitespace()

	p := l.peek()
	switch p {
	case ';', '(', ')', eof:
		return lexTree
	default:
		return lexProperty
	}
}
