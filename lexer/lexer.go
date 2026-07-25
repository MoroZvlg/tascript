package lexer

import (
	"strings"

	"github.com/MoroZvlg/tascript/token"
)

type Lexer struct {
	src          string
	col          int
	line         int
	currCursor   int
	peekCursor   int
	currChar     byte
	bracketDepth int
}

func New(input string) *Lexer {
	l := &Lexer{
		src:          input,
		col:          0,
		line:         1,
		currCursor:   -1,
		peekCursor:   0,
		currChar:     0,
		bracketDepth: 0,
	}
	l.advance()
	return l
}

func (l *Lexer) advance() {
	if l.currChar == '\n' || (l.currChar == '\r' && l.peek() != '\n') {
		l.line++
		l.col = 0
	}
	l.col++
	l.currCursor++
	l.peekCursor++
	if l.eof() {
		l.currChar = 0
		return
	}
	l.currChar = l.src[l.currCursor]
}

func (l *Lexer) peek() byte {
	if l.peekEof() {
		return 0
	}
	return l.src[l.peekCursor]
}

func (l *Lexer) NextToken() token.Token {
	var t token.Token

	for {
		if l.eof() {
			break
		}

		if l.isCommentStart() {
			l.skipCommentLine()
		}

		if l.atNewline() && l.bracketDepth > 0 {
			l.advance()
			continue
		}

		if l.currChar != ' ' && l.currChar != '\t' {
			break
		}

		l.advance()
	}

	if l.atNewline() {
		pos := l.pos()
		for { // collapse consecutive newlines, including blank lines with whitespace
			l.advance()

			if l.isCommentStart() {
				l.skipCommentLine()
			}

			if l.currChar == ' ' || l.currChar == '\t' || l.currChar == '\r' {
				continue
			}

			if l.eof() || l.currChar != '\n' {
				break
			}
		}
		return token.Token{Pos: pos, Type: token.NEWLINE, Literal: ""}
	}

	if l.eof() {
		return token.Token{Pos: l.pos(), Type: token.EOF, Literal: ""}
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
		dotFound := false
		for isDigit(l.currChar) || l.currChar == '.' {
			if l.currChar == '.' {
				if dotFound {
					break
				}
				dotFound = true
			}
			l.advance()
		}
		tok := token.Token{Pos: pos, Literal: l.src[startIdx:l.currCursor]}
		if dotFound {
			tok.Type = token.FLOAT
		} else {
			tok.Type = token.INTEGER
		}
		return tok
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
	case '%':
		t = token.Token{Pos: l.pos(), Type: token.PERCENT, Literal: "%"}
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
		l.bracketDepth++
		t = token.Token{Pos: l.pos(), Type: token.LPAREN, Literal: "("}
	case ')':
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		t = token.Token{Pos: l.pos(), Type: token.RPAREN, Literal: ")"}
	case '{':
		t = token.Token{Pos: l.pos(), Type: token.LBRACE, Literal: "{"}
	case '}':
		t = token.Token{Pos: l.pos(), Type: token.RBRACE, Literal: "}"}
	case '[':
		l.bracketDepth++
		t = token.Token{Pos: l.pos(), Type: token.LBRACKET, Literal: "["}
	case ']':
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		t = token.Token{Pos: l.pos(), Type: token.RBRACKET, Literal: "]"}
	case ',':
		t = token.Token{Pos: l.pos(), Type: token.COMMA, Literal: ","}
	case ':':
		t = token.Token{Pos: l.pos(), Type: token.COLON, Literal: ":"}
	case 0:
		t = token.Token{Pos: l.pos(), Type: token.ILLEGAL, Literal: "illegal NUL byte"}
	case '"':
		pos := l.pos()
		lit, errMsg := l.readString()
		if errMsg != "" {
			t = token.Token{Pos: pos, Type: token.ILLEGAL, Literal: errMsg}
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

func (l *Lexer) readString() (string, string) {
	var sb strings.Builder
	errMsg := "" // first error wins; we keep scanning to the closing quote either way
	for {
		// stop before the newline, never on it: the parser needs it as a statement separator
		if isNewline(l.peek()) {
			return "", "unterminated string"
		}
		l.advance()
		switch l.currChar {
		case 0:
			if l.eof() {
				if errMsg == "" {
					errMsg = "unterminated string"
				}
				return "", errMsg
			}
			// embedded NUL (not end-of-input): illegal, same as outside a string
			if errMsg == "" {
				errMsg = "illegal NUL byte"
			}
		case '"':
			if errMsg != "" {
				return "", errMsg
			}
			return sb.String(), ""
		case '\\':
			if isNewline(l.peek()) {
				return "", "unterminated string"
			}
			l.advance()
			if l.eof() {
				return "", "unterminated string"
			}
			switch l.currChar {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			default:
				if errMsg == "" {
					errMsg = "invalid escape sequence"
				}
			}
		default:
			sb.WriteByte(l.currChar)
		}
	}
}

func (l *Lexer) isCommentStart() bool {
	return l.currChar == '/' && l.peek() == '/'
}

func (l *Lexer) skipCommentLine() {
	for {
		if l.eof() || l.atNewline() {
			break
		}
		l.advance()
	}
}

func (l *Lexer) pos() token.Pos {
	return token.Pos{
		Line: l.line,
		Col:  l.col,
	}
}

func (l *Lexer) atNewline() bool { return isNewline(l.currChar) }

func isNewline(char byte) bool { return char == '\n' || char == '\r' }

func (l *Lexer) eof() bool { return l.currCursor >= len(l.src) }

func (l *Lexer) peekEof() bool { return l.peekCursor >= len(l.src) }

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || (ch == '_')
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
