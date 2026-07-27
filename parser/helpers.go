package parser

import (
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/token"
)

func (p *Parser) syncToNewLine() {
	for {
		if p.currTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) {
			break
		}
		if p.peekTokenIs(token.NEWLINE) {
			p.nextToken()
			break
		}
		p.nextToken()
	}
}

// syncToStmtEnd recovers to the next  `}`, NEWLINE or EOF.
func (p *Parser) syncToStmtEnd() {
	depth := 0
	for {
		if p.currTokenIs(token.EOF) {
			return
		}
		if depth == 0 {
			if p.currTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.RBRACE) {
				return
			}
			if p.peekTokenIs(token.NEWLINE) {
				p.nextToken()
				return
			}
		}
		switch p.peekToken.Type {
		case token.LBRACE:
			depth++
		case token.RBRACE:
			depth--
		}
		p.nextToken()
	}
}

// advanceToLBrace true if `{` found. Otherwise false
func (p *Parser) advanceToLBrace() bool {
	for !p.currTokenIs(token.LBRACE) {
		if p.currTokenIs(token.NEWLINE) || p.currTokenIs(token.RBRACE) || p.currTokenIs(token.EOF) {
			return false
		}
		p.nextToken()
	}
	return true
}

func (p *Parser) skipPeekNewLines() {
	for p.peekTokenIs(token.NEWLINE) {
		p.nextToken()
	}
}

func (p *Parser) addExpressionExpected(tok token.Token) {
	p.addDiag(&diag.ExpressionExpected{
		At:  tok.Pos,
		Got: tok.Type,
	})
}

func (p *Parser) addUnexpectedToken(tok token.Token, expected token.TokenType) {
	p.addDiag(&diag.UnexpectedToken{
		At:       tok.Pos,
		Got:      tok.Type,
		Expected: expected,
	})
}

func (p *Parser) addTopDeclUnexpected(pos token.Pos) {
	p.addDiag(&diag.TopDeclUnexpected{
		At: pos,
	})
}

func (p *Parser) addTopDeclMisplaced(tok token.Token) {
	p.addDiag(&diag.TopDeclMisplaced{
		At:      tok.Pos,
		Keyword: tok.Type,
	})
}

func (p *Parser) addDuplicateDecl(kwToken, identToken token.Token) {
	p.addDiag(&diag.DuplicateDeclaration{
		At:      kwToken.Pos,
		Keyword: kwToken.Literal,
		Name:    identToken.Literal,
	})
}

func (p *Parser) addNumberOutOfRange(tok token.Token) {
	p.addDiag(&diag.NumberOutOfRange{
		At:      tok.Pos,
		Target:  tok.Type,
		Literal: tok.Literal,
	})
}

func (p *Parser) addTypeExpected(pos token.Pos) {
	p.addDiag(&diag.TypeExpected{
		At: pos,
	})
}

func (p *Parser) addEmptyCustomType(pos token.Pos) {
	p.addDiag(&diag.EmptyCustomType{
		At: pos,
	})
}

func (p *Parser) addForbiddenFunction(pos token.Pos) {
	p.addDiag(&diag.ForbiddenFunction{
		At: pos,
	})
}

func (p *Parser) addEmptyFunction(pos token.Pos) {
	p.addDiag(&diag.EmptyFunction{
		At: pos,
	})
}

func (p *Parser) addArgOrderInvalid(pos token.Pos) {
	p.addDiag(&diag.ArgOrderInvalid{
		At: pos,
	})
}

func (p *Parser) addMissingRun(pos token.Pos) {
	p.addDiag(&diag.MissingRun{
		At: pos,
	})
}

func (p *Parser) addNestingTooDeep(pos token.Pos) {
	p.addDiag(&diag.NestingTooDeep{
		At: pos,
	})
}

func (p *Parser) nextToken() {
	p.currentToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) currTokenIs(t token.TokenType) bool {
	return p.currentToken.Type == t
}

func (p *Parser) peekPrecedence() precedence {
	prec, ok := precedences[p.peekToken.Type]
	if !ok {
		return 0
	}
	return prec
}

func (p *Parser) currentPrecedence() precedence {
	prec, ok := precedences[p.currentToken.Type]
	if !ok {
		return 0
	}
	return prec
}

func isClosingDelim(t token.TokenType) bool {
	return t == token.RPAREN || // )
		t == token.RBRACKET || // ]
		t == token.RBRACE || // }
		t == token.COMMA // ,
}
