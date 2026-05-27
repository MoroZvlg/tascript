package lexer

import (
	"fmt"

	"github.com/MoroZvlg/tascript/token"
)

type Error struct {
	Pos token.Pos
	Msg string
}

func (e Error) Error() string { return fmt.Sprintf("%s: %s", e.Pos, e.Msg) }

type Lexer struct {
	src    []byte
	file   string
	cursor int // byte offset of the next byte to consume
	line   int
	col    int
	errors []Error
}

func New(src []byte, file string) *Lexer {
	return &Lexer{src: src, file: file, line: 1, col: 1}
}

func (l *Lexer) Errors() []Error { return l.errors }

func (l *Lexer) Tokenize() []token.Token {
	var out []token.Token
	for {
		t := l.Next()
		out = append(out, t)
		if t.Kind == token.EOF {
			return out
		}
	}
}

func (l *Lexer) Next() token.Token {
	l.skipBlanks()

	if l.eof() {
		return l.tok(token.EOF, "", l.pos())
	}

	currChar := l.peek()

	if currChar == '\n' {
		p := l.pos()
		for !l.eof() && (l.peek() == '\n' || l.peek() == ' ' || l.peek() == '\t' || l.startsComment()) {
			if l.startsComment() {
				l.skipLineComment()
				continue
			}

			l.advance()
		}
		return l.tok(token.NEWLINE, "\n", p)
	}

	if isLetter(currChar) || currChar == '_' {
		return l.readIdent()
	}
	if isDigit(currChar) {
		return l.readNumber()
	}
	if currChar == '"' {
		return l.readString()
	}

	p := l.pos()
	l.advance()
	switch currChar {
	case '=':
		if l.expectNext('=') {
			return l.tok(token.EQ, "==", p)
		}
		return l.tok(token.ASSIGN, "=", p)
	case ':':
		return l.tok(token.COLON, ":", p)
	case ',':
		return l.tok(token.COMMA, ",", p)
	case '.':
		return l.tok(token.DOT, ".", p)
	case '+':
		return l.tok(token.PLUS, "+", p)
	case '-':
		return l.tok(token.MINUS, "-", p)
	case '*':
		return l.tok(token.ASTERISK, "*", p)
	case '/':
		return l.tok(token.SLASH, "/", p)
	case '%':
		return l.tok(token.PERCENT, "%", p)
	case '!':
		if l.expectNext('=') {
			return l.tok(token.NEQ, "!=", p)
		}
		return l.tok(token.BANG, "!", p)
	case '<':
		if l.expectNext('=') {
			return l.tok(token.LTE, "<=", p)
		}
		return l.tok(token.LT, "<", p)
	case '>':
		if l.expectNext('=') {
			return l.tok(token.GTE, ">=", p)
		}
		return l.tok(token.GT, ">", p)
	case '&':
		if l.expectNext('&') {
			return l.tok(token.AND, "&&", p)
		}
		l.addErrf(p, "unexpected '&'; use '&&' for logical and")
		return l.tok(token.ILLEGAL, "&", p)

	case '|':
		if l.expectNext('|') {
			return l.tok(token.OR, "||", p)
		}
		l.addErrf(p, "unexpected '|'; use '||' for logical or")
		return l.tok(token.ILLEGAL, "|", p)
	case '(':
		return l.tok(token.LPAREN, "(", p)
	case ')':
		return l.tok(token.RPAREN, ")", p)
	case '{':
		return l.tok(token.LBRACE, "{", p)
	case '}':
		return l.tok(token.RBRACE, "}", p)
	case '[':
		return l.tok(token.LBRACKET, "[", p)
	case ']':
		return l.tok(token.RBRACKET, "]", p)
	}
	l.addErrf(p, "unexpected character %q", currChar)
	return l.tok(token.ILLEGAL, string(currChar), p)
}

func (l *Lexer) skipBlanks() {
	for !l.eof() {
		currChar := l.peek()
		if currChar == ' ' || currChar == '\t' || currChar == '\r' {
			l.advance()
			continue
		}
		if l.startsComment() {
			l.skipLineComment()
			continue
		}
		return
	}
}

func (l *Lexer) startsComment() bool {
	return l.cursor+1 < len(l.src) && l.src[l.cursor] == '/' && l.src[l.cursor+1] == '/'
}

func (l *Lexer) skipLineComment() {
	for !l.eof() && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) readIdent() token.Token {
	p := l.pos()
	start := l.cursor
	for !l.eof() {
		currChar := l.peek()
		if isLetter(currChar) || isDigit(currChar) || currChar == '_' {
			l.advance()
			continue
		}
		break
	}
	return l.tok(token.IDENT, string(l.src[start:l.cursor]), p)
}

func (l *Lexer) readNumber() token.Token {
	p := l.pos()
	start := l.cursor
	for !l.eof() && isDigit(l.peek()) {
		l.advance()
	}
	if !l.eof() && l.peek() == '.' && l.cursor+1 < len(l.src) && isDigit(l.src[l.cursor+1]) {
		l.advance()
		for !l.eof() && isDigit(l.peek()) {
			l.advance()
		}
	}
	return l.tok(token.NUMBER, string(l.src[start:l.cursor]), p)
}

func (l *Lexer) readString() token.Token {
	p := l.pos()
	l.advance() // opening "
	start := l.cursor
	for !l.eof() && l.peek() != '"' && l.peek() != '\n' {
		l.advance()
	}
	if l.eof() || l.peek() == '\n' {
		l.addErrf(p, "unterminated string literal")
		return l.tok(token.ILLEGAL, string(l.src[start:l.cursor]), p)
	}
	lit := string(l.src[start:l.cursor])
	l.advance() // closing "
	return l.tok(token.STRING, lit, p)
}

func (l *Lexer) pos() token.Pos { return token.Pos{File: l.file, Line: l.line, Column: l.col} }

func (l *Lexer) tok(k token.Kind, lit string, p token.Pos) token.Token {
	return token.Token{Kind: k, Literal: lit, Pos: p}
}

func (l *Lexer) eof() bool  { return l.cursor >= len(l.src) }
func (l *Lexer) peek() byte { return l.src[l.cursor] }

func (l *Lexer) expectNext(want byte) bool {
	if l.eof() || l.peek() != want {
		return false
	}
	l.advance()
	return true
}

func (l *Lexer) advance() {
	if l.cursor >= len(l.src) {
		return
	}
	if l.src[l.cursor] == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.cursor++
}

func (l *Lexer) addErrf(p token.Pos, format string, args ...any) {
	l.errors = append(l.errors, Error{Pos: p, Msg: fmt.Sprintf(format, args...)})
}

func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }
func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
