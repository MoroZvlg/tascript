package parser

import (
	"strconv"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/token"
)

func (p *Parser) parseExpression(prec precedence) ast.Expression {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxNestingDepth {
		p.addNestingTooDeep(p.currentToken.Pos)
		return &ast.BadExpr{Token: p.currentToken}
	}

	prefFn, ok := p.prefixFns[p.currentToken.Type]
	if !ok {
		p.addExpressionExpected(p.currentToken)
		return &ast.BadExpr{Token: p.currentToken}
	}
	leftExpr := prefFn()
	// bad leftExpr - no further parsing. Protects against repeated NestingTooDeep errors
	for !ast.IsBadExpr(leftExpr) && !p.peekTokenIs(token.NEWLINE) && prec < p.peekPrecedence() {
		infixExprFn, okInf := p.infixFns[p.peekToken.Type]
		if !okInf {
			return leftExpr
		}
		p.nextToken()
		leftExpr = infixExprFn(leftExpr)
	}
	return leftExpr
}

func (p *Parser) parsePrefixExpr() ast.Expression {
	expr := &ast.PrefixExpr{
		Token: p.currentToken,
	}
	if isClosingDelim(p.peekToken.Type) || p.peekTokenIs(token.EOF) {
		p.addExpressionExpected(p.peekToken)
		expr.Right = &ast.BadExpr{Token: p.peekToken}
		return expr
	}
	p.nextToken()
	expr.Right = p.parseExpression(PrefixPrec)
	return expr
}

func (p *Parser) parseInfixExpr(left ast.Expression) ast.Expression {
	// left may be a BadExpr (e.g. overflow); carry it as-is, its error is already reported.
	expr := ast.InfixExpr{Token: p.currentToken, Left: left}
	prec := p.currentPrecedence()
	if isClosingDelim(p.peekToken.Type) || p.peekTokenIs(token.EOF) {
		expr.Right = &ast.BadExpr{Token: p.currentToken}
		p.addExpressionExpected(p.peekToken)
		return &expr
	}
	p.nextToken()
	// a bad RHS already reports itself inside parseExpression, no extra error here.
	expr.Right = p.parseExpression(prec)
	return &expr
}

func (p *Parser) parseGroupExpr() ast.Expression {
	p.nextToken() // skip `(`
	expr := p.parseExpression(LowestPrec)
	if ast.IsBadExpr(expr) { // already reported by the inner expression
		return expr
	}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // skip `)`
	} else {
		p.addUnexpectedToken(p.peekToken, token.RPAREN)
		return &ast.BadExpr{Token: p.currentToken}
	}

	return expr
}

func (p *Parser) parseMemberExpr(left ast.Expression) ast.Expression {
	expr := ast.MemberAccessExpr{Token: p.currentToken, Object: left}
	p.nextToken() // advance past `.` onto the member name
	if !p.currTokenIs(token.IDENT) {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
		return &ast.BadExpr{Token: p.currentToken}
	}
	expr.Member = &ast.IdentExpr{Token: p.currentToken}
	return &expr
}

func (p *Parser) parseIndexExpr(left ast.Expression) ast.Expression {
	expr := &ast.IndexExpr{Token: p.currentToken, Left: left}
	p.nextToken()
	expr.Index = p.parseExpression(LowestPrec)
	if ast.IsBadExpr(expr.Index) {
		return expr.Index
	}

	if p.peekTokenIs(token.RBRACKET) {
		p.nextToken() // consume `]`
	} else {
		p.addUnexpectedToken(p.peekToken, token.RBRACKET)
		return &ast.BadExpr{Token: expr.Token}
	}
	return expr
}

func (p *Parser) parseCallExpr(callee ast.Expression) ast.Expression {
	callExpr := &ast.CallExpr{Token: p.currentToken, Callee: callee}
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken() // consume `(`
		return callExpr
	}
	kwargSeen := false
	for {
		p.nextToken()

		errsBefore := p.errCount
		if p.currTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			kwargSeen = true
			kwarg := &ast.KwargsExpr{Token: p.currentToken, Key: &ast.IdentExpr{Token: p.currentToken}}
			p.nextToken() // consume `=`
			p.nextToken()
			value := p.parseExpression(LowestPrec)
			if p.errCount == errsBefore {
				kwarg.Value = value
				callExpr.Kwargs = append(callExpr.Kwargs, kwarg)
			}
		} else {
			if kwargSeen {
				p.addArgsOrder(p.currentToken.Pos)
				break
			}
			arg := p.parseExpression(LowestPrec)
			if p.errCount == errsBefore {
				callExpr.Args = append(callExpr.Args, arg)
			}
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RPAREN) {
				break
			}
			continue
		}

		break
	}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return callExpr
	}
	p.addUnexpectedToken(p.peekToken, token.RPAREN)
	return &ast.BadExpr{Token: callExpr.Token}
}

func (p *Parser) parseIdentExpr() ast.Expression {
	return &ast.IdentExpr{Token: p.currentToken}
}

func (p *Parser) parseIntegerExpr() ast.Expression {
	val, err := strconv.Atoi(p.currentToken.Literal)
	if err != nil {
		p.addParseFailed(p.currentToken.Pos, p.currentToken.Type, err)
		return &ast.BadExpr{Token: p.currentToken}
	}
	return &ast.IntegerExpr{Token: p.currentToken, Value: val}
}

func (p *Parser) parseFloatExpr() ast.Expression {
	val, err := strconv.ParseFloat(p.currentToken.Literal, 64)
	if err != nil {
		p.addParseFailed(p.currentToken.Pos, p.currentToken.Type, err)
		return &ast.BadExpr{Token: p.currentToken}
	}
	return &ast.FloatExpr{Token: p.currentToken, Value: val}
}

func (p *Parser) parseStringExpr() ast.Expression {
	return &ast.StringExpr{Token: p.currentToken, Value: p.currentToken.Literal}
}

func (p *Parser) parseBoolExpr() ast.Expression {
	val, _ := strconv.ParseBool(p.currentToken.Literal)
	return &ast.BooleanExpr{Token: p.currentToken, Value: val}
}
