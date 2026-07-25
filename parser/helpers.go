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
		Phase: diag.PhaseParse,
		Pos:   tok.Pos,
		Got:   tok.Type,
	})
}

func (p *Parser) addUnexpectedToken(tok token.Token, expected token.TokenType) {
	p.addDiag(&diag.UnexpectedToken{
		Phase:    diag.PhaseParse,
		Pos:      tok.Pos,
		Got:      tok.Type,
		Expected: expected,
	})
}

func (p *Parser) addUnexpectedTopDecl(pos token.Pos) {
	p.addDiag(&diag.UnexpectedTopDecl{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addTopDeclInBody(tok token.Token) {
	p.addDiag(&diag.TopDeclInBody{
		Phase:   diag.PhaseParse,
		Pos:     tok.Pos,
		Keyword: tok.Type,
	})
}

func (p *Parser) addDuplicateDecl(kwToken, identToken token.Token) {
	p.addDiag(&diag.DuplicateDeclaration{
		Phase:        diag.PhaseParse,
		KeywordToken: kwToken,
		IdentToken:   identToken,
	})
}

func (p *Parser) addParseFailed(pos token.Pos, target token.TokenType, _ error) {
	p.addDiag(&diag.ParseFailed{
		Phase:  diag.PhaseParse,
		Pos:    pos,
		Target: target,
	})
}

func (p *Parser) addTypeOrCustomTypeExpected(pos token.Pos) {
	p.addDiag(&diag.TypeOrCustomTypeExpected{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addEmptyCustomType(pos token.Pos) {
	p.addDiag(&diag.EmptyCustomType{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addForbiddenFunc(pos token.Pos) {
	p.addDiag(&diag.ForbiddenFunc{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addEmptyFunctionBody(pos token.Pos) {
	p.addDiag(&diag.EmptyFunctionBody{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addArgsOrder(pos token.Pos) {
	p.addDiag(&diag.ArgsOrder{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addMissingRunFn(pos token.Pos) {
	p.addDiag(&diag.MissingRunFunc{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addNestingTooDeep(pos token.Pos) {
	p.addDiag(&diag.NestingTooDeep{
		Phase: diag.PhaseParse,
		Pos:   pos,
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
