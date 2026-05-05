package lexer

import (
	"github.com/MoroZvlg/tascript/token"
)

type Lexer struct {
	input        string
	position     int
	readPosition int
	char         byte
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.char = 0
	} else {
		l.char = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

func (l *Lexer) NextToken() token.Token {
	var t token.Token

	l.skipWhitespaces()

	switch l.char {
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			t = token.Token{Type: token.EQ, Literal: "=="}
		} else {
			t = token.Token{Type: token.ASSIGN, Literal: "="}
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			t = token.Token{Type: token.LTEQ, Literal: "<="}
		} else {
			t = token.Token{Type: token.LT, Literal: "<"}
		}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			t = token.Token{Type: token.GTEQ, Literal: ">="}
		} else {
			t = token.Token{Type: token.GT, Literal: ">"}
		}
	case '+':
		t = token.Token{Type: token.PLUS, Literal: "+"}
	case '-':
		t = token.Token{Type: token.MINUS, Literal: "-"}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			t = token.Token{Type: token.NEQ, Literal: "!="}
		} else {
			t = token.Token{Type: token.BANG, Literal: "!"}
		}
	case '*':
		t = token.Token{Type: token.ASTERISK, Literal: "*"}
	case '/':
		if l.peekChar() == '/' {
			l.skipComment()
			return l.NextToken() // TODO: Recursive?? is it ok??
		}
		t = token.Token{Type: token.SLASH, Literal: "/"}
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			t = token.Token{Type: token.AND, Literal: "&&"}
		} else {
			t = token.Token{Type: token.ILLEGAL, Literal: string(l.char)}
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			t = token.Token{Type: token.OR, Literal: "||"}
		} else {
			t = token.Token{Type: token.ILLEGAL, Literal: string(l.char)}
		}
	case '(':
		t = token.Token{Type: token.LPAREN, Literal: "("}
	case ')':
		t = token.Token{Type: token.RPAREN, Literal: ")"}
	case '{':
		t = token.Token{Type: token.LBRACE, Literal: "{"}
	case '}':
		t = token.Token{Type: token.RBRACE, Literal: "}"}
	case '[':
		t = token.Token{Type: token.LBRACKET, Literal: "["}
	case ']':
		t = token.Token{Type: token.RBRACKET, Literal: "]"}
	case ',':
		t = token.Token{Type: token.COMMA, Literal: ","}
	case ':':
		t = token.Token{Type: token.COLON, Literal: ":"}
	case ';':
		t = token.Token{Type: token.SEMICOLON, Literal: ";"}
	case 0:
		t = token.Token{Type: token.EOF, Literal: ""}
	case '"':
		lit, ok := l.readString()
		if !ok {
			t = token.Token{Type: token.ILLEGAL, Literal: "unterminated string"}
		} else {
			t = token.Token{Type: token.STRING, Literal: lit}
		}
	case '.':
		t = token.Token{Type: token.DOT, Literal: "."}
	default:
		if isLetter(l.char) {
			lit := l.readIdentifier()
			return token.Token{Type: token.LookupIdent(lit), Literal: lit}
		} else if isDigit(l.char) {
			tt, lit := l.readNumber()
			return token.Token{Type: tt, Literal: lit}
		}
		t = token.Token{Type: token.ILLEGAL, Literal: string(l.char)}
	}
	l.readChar()
	return t
}

func (l *Lexer) skipWhitespaces() {
	for {
		if l.char != ' ' && l.char != '\n' && l.char != '\t' && l.char != '\r' {
			break
		}
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	startPos := l.position
	for isLetter(l.char) {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

func (l *Lexer) readNumber() (token.TokenType, string) {
	startPos := l.position
	tt := token.INT
	for isDigit(l.char) {
		l.readChar()
	}
	if l.char == '.' && isDigit(l.peekChar()) {
		tt = token.FLOAT
		l.readChar()
		for isDigit(l.char) {
			l.readChar()
		}
	}
	return tt, l.input[startPos:l.position]
}

func (l *Lexer) readString() (string, bool) {
	startPos := l.position + 1
	for {
		l.readChar()
		if l.char == 0 {
			return l.input[startPos:l.position], false
		}
		if l.char == '"' {
			return l.input[startPos:l.position], true
		}
	}
}

func (l *Lexer) skipComment() {
	for {
		l.readChar()
		if l.char == '\n' || l.char == 0 {
			break
		}
	}
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
