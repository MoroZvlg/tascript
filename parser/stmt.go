package parser

import (
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/token"
)

// parseBlock parses a `{ ... }` block. Precondition: current token is `{`.
func (p *Parser) parseBlock() *ast.BlockStmt {
	block := &ast.BlockStmt{Token: p.currentToken} // current is `{`
	p.nextToken()                                  // consume `{`

	for {
		if p.currTokenIs(token.NEWLINE) {
			p.nextToken()
			continue
		}
		if p.currTokenIs(token.RBRACE) {
			break
		}
		if p.currTokenIs(token.EOF) {
			p.addUnexpectedToken(p.currentToken, token.RBRACE) // the opening `{` was never closed
			break
		}

		errsBeforeStmt := p.errCount
		block.Stmts = append(block.Stmts, p.parseStatement())
		switch {
		case p.errCount != errsBeforeStmt:
			p.syncToStmtEnd() // already reported; just recover position
		case !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF):
			p.addUnexpectedToken(p.peekToken, token.NEWLINE)
			p.syncToStmtEnd()
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.currentToken.Type {
	case token.LET:
		return p.parseLetStmt()
	case token.IF:
		return p.parseIfStmt()
	case token.CONST, token.INPUT, token.OUTPUT:
		p.addTopDeclMisplaced(p.currentToken)
		return &ast.BadStmt{From: p.currentToken.Pos, To: p.currentToken.Pos}
	default:
		return p.parseExprOrAssignStmt()
	}
}

func (p *Parser) parseIfStmt() ast.Statement {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxNestingDepth {
		p.addNestingTooDeep(p.currentToken.Pos)
		return &ast.BadStmt{From: p.currentToken.Pos, To: p.currentToken.Pos}
	}

	ifTok := p.currentToken
	stmt := &ast.IfStmt{Token: ifTok}
	beforeCondErrs := p.errCount

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // current = `(`
		p.nextToken() // current = first condition token
		stmt.Condition = p.parseExpression(LowestPrec)
		if p.peekTokenIs(token.RPAREN) {
			p.nextToken() // current = `)`
		} else if p.errCount == beforeCondErrs {
			p.addUnexpectedToken(p.peekToken, token.RPAREN)
		}
	} else {
		// didn't find condition. but will try to parse blocks anyway
		p.addUnexpectedToken(p.peekToken, token.LPAREN)
	}

	if p.peekTokenIs(token.LBRACE) {
		p.nextToken() // current = `{`
	} else {
		if p.errCount == beforeCondErrs {
			p.addUnexpectedToken(p.peekToken, token.LBRACE)
		}
		if !p.advanceToLBrace() {
			return &ast.BadStmt{From: ifTok.Pos, To: p.currentToken.Pos}
		}
	}
	stmt.Consequence = p.parseBlock() // leaves current ON `}`

	if !p.peekTokenIs(token.ELSE) {
		return stmt
	}
	p.nextToken() // current = `else`

	switch {
	case p.peekTokenIs(token.IF): // else if — the nested if becomes the Else stmt directly
		p.nextToken() // current = `if`
		stmt.Else = p.parseIfStmt()
	case p.peekTokenIs(token.LBRACE):
		p.nextToken() // current = `{`
		stmt.Else = p.parseBlock()
	default:
		p.addUnexpectedToken(p.peekToken, token.LBRACE)
		if p.advanceToLBrace() {
			stmt.Else = p.parseBlock()
		}
	}
	return stmt
}

func (p *Parser) parseExprOrAssignStmt() ast.Statement {
	tok := p.currentToken
	expr := p.parseExpression(LowestPrec)

	if !p.peekTokenIs(token.ASSIGN) {
		return &ast.ExprStmt{Token: tok, Expr: expr}
	}

	p.nextToken() // current = `=`
	stmt := &ast.AssignStmt{Token: p.currentToken, Target: expr}
	p.nextToken() // current = first RHS token
	stmt.Value = p.parseExpression(LowestPrec)
	return stmt
}

func (p *Parser) parseLetStmt() *ast.LetStmt {
	letStmt := &ast.LetStmt{Token: p.currentToken}

	p.nextToken()

	if p.currTokenIs(token.IDENT) {
		letStmt.Name = &ast.IdentExpr{Token: p.currentToken}
		p.nextToken()
	} else {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
	}

	if p.currTokenIs(token.ASSIGN) {
		p.nextToken()
	} else {
		if letStmt.Name != nil {
			p.addUnexpectedToken(p.currentToken, token.ASSIGN)
		}
	}

	letStmt.Value = p.parseExpression(LowestPrec)

	return letStmt
}
