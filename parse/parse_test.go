package parse

import (
	"fmt"
	"testing"
	"time"
)

type expectedItem struct {
	item      item
	ignoreVal bool
}

func checkLexerOutput(t *testing.T, expected []expectedItem, l *lexer) {
	for i, v := range expected {
		select {
		case lv := <-l.items:
			if v.item.typ != lv.typ {
				t.Errorf("%v[%d]: expected type: %v, got: %v", l.name, i, v.item.typ, lv.typ, lv)
			}
			if !v.ignoreVal {
				var notequal bool
				if len(v.item.val) != len(lv.val) {
					notequal = true
				}
				if !notequal {
					for i, v := range v.item.val {
						if v != lv.val[i] {
							notequal = true
							break
						}
					}
				}
				if notequal {
					t.Errorf("%v[%d]: expected value: '%s', got: '%s'", l.name, i, v.item.val, lv.val)
				}
			}
		case <-time.After(3 * time.Second):
			fmt.Println("timeout!")
			break
		}
	}
	if _, ok := <-l.items; ok {
		t.Errorf("%v: more items than expected", l.name)
	}
}

func TestLexer(t *testing.T) {
	type testset struct {
		name string
		sgf  string
		exp  []expectedItem
	}

	dontcare := []byte{}

	pairs := []testset{
		{
			name: "empty string",
			sgf:  "",
			exp: []expectedItem{
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "junk string",
			sgf:  " asdf asdf ",
			exp: []expectedItem{
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "empty gametree",
			sgf:  "()",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "empty gametree with space",
			sgf:  "  (  )  ",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "two empty gametrees",
			sgf:  "()()",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "nested empty gametrees",
			sgf:  "(())",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "unclosed gametree",
			sgf:  "(",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemError, val: []byte("unexpected EOF")}, false},
			},
		},
		{
			name: "too many closing parens",
			sgf:  "())",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemError, val: []byte("too many right parentheses")}, false},
			},
		},
		{
			name: "simple nodes",
			sgf:  "(;A[]BX[1];C[hello [world\\] 世界];DEF[])",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemPropertyIdent, val: []byte("A")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte{}}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},
				{item{typ: itemPropertyIdent, val: []byte("BX")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("1")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemPropertyIdent, val: []byte("C")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("hello [world\\] 世界")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemWarning, val: []byte("Found PropertyIdent wider than 2 characters")}, false},
				{item{typ: itemPropertyIdent, val: []byte("DEF")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte{}}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},
				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
		{
			name: "malformed PropertyIdent",
			sgf:  "(;a[])",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemError, val: []byte("PropertyIdent must be upper-case letters")}, false},
			},
		},
		{
			name: "unfinished PropertyIdent",
			sgf:  "(;A",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemError, val: []byte("unexpected EOF")}, false},
			},
		},
		{
			name: "unfinished PropertyValue",
			sgf:  "(;A[",
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},
				{item{typ: itemSemiColon, val: dontcare}, true},
				{item{typ: itemPropertyIdent, val: []byte("A")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemError, val: []byte("unexpected EOF")}, false},
			},
		},
		{
			name: "something realistic",
			sgf: `
			(
				;FF[4]
				GM[1]
				SZ[19]
				;B[aa];W[bb]
				(;B[cc];W[dd];B[ff])
				(;B[ll])
			)
			`,
			exp: []expectedItem{
				{item{typ: itemOpenParen, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("FF")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("4")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("GM")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("1")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("SZ")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("19")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("B")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("aa")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("W")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("bb")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemOpenParen, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("B")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("cc")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("W")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("dd")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("B")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("ff")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemCloseParen, val: dontcare}, true},

				{item{typ: itemOpenParen, val: dontcare}, true},

				{item{typ: itemSemiColon, val: dontcare}, true},

				{item{typ: itemPropertyIdent, val: []byte("B")}, false},
				{item{typ: itemOpenBracket, val: dontcare}, true},
				{item{typ: itemPropertyValue, val: []byte("ll")}, false},
				{item{typ: itemCloseBracket, val: dontcare}, true},

				{item{typ: itemCloseParen, val: dontcare}, true},

				{item{typ: itemCloseParen, val: dontcare}, true},
				{item{typ: itemEOF, val: dontcare}, true},
			},
		},
	}
	for _, pair := range pairs {
		l := lex(pair.name, []byte(pair.sgf))
		checkLexerOutput(t, pair.exp, l)
	}
}
