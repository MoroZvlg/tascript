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

func (l *Lexer) NextToken() token.Token {
	var t token.Token

	l.skipWhitespaces()

	switch l.char {
	// 1 char tokens
	case '=':
		t = token.Token{Type: token.ASSIGN, Literal: "="}
	case '<':
		t = token.Token{Type: token.LT, Literal: "<"}
	case '>':
		t = token.Token{Type: token.GT, Literal: ">"}
	case '+':
		t = token.Token{Type: token.PLUS, Literal: "+"}
	case '-':
		t = token.Token{Type: token.MINUS, Literal: "-"}
	case '!':
		t = token.Token{Type: token.BANG, Literal: "!"}
	case '*':
		t = token.Token{Type: token.ASTERISK, Literal: "*"}
	case '/':
		t = token.Token{Type: token.SLASH, Literal: "/"}
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
	default:
		if isLetter(l.char) {
			lit := l.readIdentifier()
			return token.Token{Type: token.LookupIdent(lit), Literal: lit}
		} else if isDigit(l.char) {
			lit := l.readNumber()
			return token.Token{Type: token.INT, Literal: lit}
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

func (l *Lexer) readNumber() string {
	startPos := l.position
	for isDigit(l.char) {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
