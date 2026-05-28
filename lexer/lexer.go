package lexer

import (
	"github.com/MoroZvlg/tascript/token"
)

type Lexer struct {
	src        string
	col        int
	line       int
	currCursor int
	peekCursor int
	currChar   byte
}

func New(input string) *Lexer {
	l := &Lexer{
		src:        input,
		col:        0,
		line:       1,
		currCursor: -1,
		peekCursor: 0,
		currChar:   0,
	}
	l.advance()
	return l
}

func (l *Lexer) advance() {
	if l.eof() {
		l.currCursor++
		l.currChar = 0
		return
	}
	if l.currChar == '\n' {
		l.line++
		l.col = 0
	}
	l.col++
	l.currCursor++
	l.peekCursor++
	l.currChar = l.src[l.currCursor]
}

func (l *Lexer) peek() byte {
	if l.eof() {
		return 0
	}
	return l.src[l.peekCursor]
}

func (l *Lexer) NextToken() token.Token {
	var t token.Token

	l.skipBlanks()

	if l.eof() {
		return token.Token{Pos: l.pos(), Type: token.EOF, Literal: ""}
	}

	if l.currChar == '\n' {
		pos := l.pos()
		l.skipBlanks()
		l.advance()
		return token.Token{Pos: pos, Type: token.NEWLINE, Literal: ""}
	}

	if isLetter(l.currChar) {
		pos := l.pos()
		startIdx := l.currCursor
		for isLetter(l.currChar) || isDigit(l.currChar) {
			l.advance()
		}
		literal := l.src[startIdx:l.currCursor]
		return token.Token{Pos: pos, Type: token.LookupIdent(literal), Literal: literal}
	}

	if isDigit(l.currChar) {
		pos := l.pos()
		startIdx := l.currCursor
		for isDigit(l.currChar) || l.currChar == '.' {
			l.advance()
		}
		return token.Token{Pos: pos, Type: token.NUMBER, Literal: l.src[startIdx:l.currCursor]}
	}

	switch l.currChar {
	case '=':
		pos := l.pos()
		if l.peek() == '=' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.EQ, Literal: "=="}
		} else {
			t = token.Token{Pos: pos, Type: token.ASSIGN, Literal: "="}
		}
	case '<':
		pos := l.pos()
		if l.peek() == '=' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.LTEQ, Literal: "<="}
		} else {
			t = token.Token{Pos: pos, Type: token.LT, Literal: "<"}
		}
	case '>':
		pos := l.pos()
		if l.peek() == '=' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.GTEQ, Literal: ">="}
		} else {
			t = token.Token{Pos: pos, Type: token.GT, Literal: ">"}
		}
	case '+':
		t = token.Token{Pos: l.pos(), Type: token.PLUS, Literal: "+"}
	case '-':
		t = token.Token{Pos: l.pos(), Type: token.MINUS, Literal: "-"}
	case '!':
		pos := l.pos()
		if l.peek() == '=' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.NEQ, Literal: "!="}
		} else {
			t = token.Token{Pos: pos, Type: token.BANG, Literal: "!"}
		}
	case '*':
		t = token.Token{Pos: l.pos(), Type: token.ASTERISK, Literal: "*"}
	case '/':
		t = token.Token{Pos: l.pos(), Type: token.SLASH, Literal: "/"}
	case '&':
		pos := l.pos()
		if l.peek() == '&' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.AND, Literal: "&&"}
		} else {
			t = token.Token{Pos: pos, Type: token.ILLEGAL, Literal: string(l.currChar)}
		}
	case '|':
		pos := l.pos()
		if l.peek() == '|' {
			l.advance()
			t = token.Token{Pos: pos, Type: token.OR, Literal: "||"}
		} else {
			t = token.Token{Pos: pos, Type: token.ILLEGAL, Literal: string(l.currChar)}
		}
	case '(':
		t = token.Token{Pos: l.pos(), Type: token.LPAREN, Literal: "("}
	case ')':
		t = token.Token{Pos: l.pos(), Type: token.RPAREN, Literal: ")"}
	case '{':
		t = token.Token{Pos: l.pos(), Type: token.LBRACE, Literal: "{"}
	case '}':
		t = token.Token{Pos: l.pos(), Type: token.RBRACE, Literal: "}"}
	case '[':
		t = token.Token{Pos: l.pos(), Type: token.LBRACKET, Literal: "["}
	case ']':
		t = token.Token{Pos: l.pos(), Type: token.RBRACKET, Literal: "]"}
	case ',':
		t = token.Token{Pos: l.pos(), Type: token.COMMA, Literal: ","}
	case ':':
		t = token.Token{Pos: l.pos(), Type: token.COLON, Literal: ":"}
	case 0:
		t = token.Token{Pos: l.pos(), Type: token.EOF, Literal: ""}
	case '"':
		pos := l.pos()
		lit, ok := l.readString()
		if !ok {
			t = token.Token{Pos: pos, Type: token.ILLEGAL, Literal: "unterminated string"}
		} else {
			t = token.Token{Pos: pos, Type: token.STRING, Literal: lit}
		}
	case '.':
		t = token.Token{Pos: l.pos(), Type: token.DOT, Literal: "."}
	default:
		t = token.Token{Pos: l.pos(), Type: token.ILLEGAL, Literal: string(l.currChar)}
	}
	l.advance()
	return t
}

func (l *Lexer) skipBlanks() {
	for {
		if l.currChar == ' ' || l.currChar == '\t' || l.currChar == '\r' {
			l.advance()
			continue
		}

		// skip empty lines (\n after \n)
		if l.currChar == '\n' && l.currCursor-1 >= 0 && l.src[l.currCursor-1] == '\n' {
			l.advance()
			continue
		}

		if l.isCommentStart() {
			l.skipCommentLine()
			continue
		}

		break
	}
}

func (l *Lexer) readString() (string, bool) {
	startIdx := l.peekCursor // skip " char
	for {
		l.advance()
		if l.currChar == 0 {
			return l.src[startIdx:l.currCursor], false
		}
		if l.currChar == '"' {
			return l.src[startIdx:l.currCursor], true
		}
	}
}

func (l *Lexer) isCommentStart() bool {
	return l.currChar == '/' && l.peek() == '/'
}

func (l *Lexer) skipCommentLine() {
	for {
		l.advance()
		if l.currChar == '\n' {
			l.advance()
			break
		}

		if l.currChar == 0 {
			break
		}
	}
}

func (l *Lexer) pos() token.Pos {
	return token.Pos{
		Line: l.line,
		Col:  l.col,
	}
}

func (l *Lexer) eof() bool { return l.peekCursor >= len(l.src) }

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
