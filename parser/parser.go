package parser

import (
	"strconv"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

const (
	InitFnIdent = "Init"
	RunFnIdent  = "Run"
)

type Parser struct {
	l            *lexer.Lexer
	currentToken token.Token
	peekToken    token.Token
	errors       []diag.Diagnostic

	prefixFns map[token.TokenType]func() ast.Expression
	infixFns  map[token.TokenType]func(ast.Expression) ast.Expression
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
	p.nextToken()
	p.nextToken()

	p.prefixFns = map[token.TokenType]func() ast.Expression{
		token.IDENT:   p.parseIdentExpr,
		token.INTEGER: p.parseIntegerExpr,
		token.FLOAT:   p.parseFloatExpr,
		token.STRING:  p.parseStringExpr,
		token.MINUS:   p.parsePrefixExpression,
		token.BANG:    p.parsePrefixExpression,
		token.TRUE:    p.parseBoolean,
		token.FALSE:   p.parseBoolean,
		token.LPAREN:  p.parseGroupExpr,
	}

	p.infixFns = map[token.TokenType]func(ast.Expression) ast.Expression{
		token.PLUS:     p.parseInfixExpr,
		token.MINUS:    p.parseInfixExpr,
		token.ASTERISK: p.parseInfixExpr,
		token.SLASH:    p.parseInfixExpr,
		token.EQ:       p.parseInfixExpr,
		token.NEQ:      p.parseInfixExpr,
		token.LT:       p.parseInfixExpr,
		token.GT:       p.parseInfixExpr,
		token.LTEQ:     p.parseInfixExpr,
		token.GTEQ:     p.parseInfixExpr,
		token.AND:      p.parseInfixExpr,
		token.OR:       p.parseInfixExpr,
		token.DOT:      p.parseCallExpr,
	}
	return p
}

func (p *Parser) Diagnostics() []diag.Diagnostic {
	return p.errors
}

func (p *Parser) Parse() *ast.Program {
	prog := &ast.Program{Valid: true}
	for !p.currTokenIs(token.EOF) {
		if p.currentToken.Type == token.NEWLINE {
			p.nextToken()
			continue
		}

		errsBefore := len(p.errors)
		switch p.currentToken.Type {
		case token.CONST:
			decl := p.parseConstDecl()
			if len(p.errors) == errsBefore {
				prog.Consts = append(prog.Consts, decl)
			}
		case token.INPUT:
			decl := p.parseInputDecl()
			if len(p.errors) == errsBefore {
				prog.Inputs = append(prog.Inputs, decl)
			}
		case token.OUTPUT:
			p.addNotImplemented(p.currentToken.Pos, "outputDecl")
		case token.FUNCTION:
			p.addNotImplemented(p.currentToken.Pos, "functionDecl")
		default:
			p.addNotImplemented(p.currentToken.Pos, "unknownDecl. Add normal error!")
		}
		if len(p.errors) == errsBefore &&
			!p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) {
			p.addUnexpectedToken(p.peekToken, token.NEWLINE)
		}
		if len(p.errors) > errsBefore {
			prog.Valid = false
			p.syncToNewLine()
		}
		p.nextToken()
	}
	return prog
}

func (p *Parser) parseInputDecl() *ast.InputDecl {
	decl := &ast.InputDecl{Token: p.currentToken}

	p.nextToken()

	if p.currTokenIs(token.IDENT) {
		decl.Identifier = &ast.IdentExpr{Token: p.currentToken}
		p.nextToken()
	} else {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
	}

	if p.currTokenIs(token.COLON) {
		p.nextToken()
	} else {
		// don't need to add second error on the same pos(we didn't move in case of missing IDEN)
		if decl.Identifier != nil {
			p.addUnexpectedToken(p.currentToken, token.COLON)
		}
		return decl
	}

	switch p.currentToken.Type {
	case token.IDENT:
		decl.Type = &ast.IdentExpr{Token: p.currentToken}
	case token.LBRACE:
		decl.Type = p.parseCustomTypeDecl()
	default:
		p.errors = append(p.errors, &diag.TypeOrCustomTypeExpected{
			Phase: diag.PhaseParse,
			Pos:   p.currentToken.Pos,
		})
	}
	return decl
}

func (p *Parser) parseConstDecl() *ast.ConstDecl {
	decl := &ast.ConstDecl{Token: p.currentToken}

	p.nextToken()

	if p.currTokenIs(token.IDENT) {
		decl.Identifier = &ast.IdentExpr{Token: p.currentToken}
		p.nextToken()
	} else {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
	}

	if p.currTokenIs(token.ASSIGN) {
		p.nextToken()
	} else {
		// don't need to add second error on the same pos(we didn't move in case of missing IDEN)
		if decl.Identifier != nil {
			p.addUnexpectedToken(p.currentToken, token.ASSIGN)
		}
		// assign already missing. If we can't parse, exit without second error
		if _, canParse := p.prefixFns[p.currentToken.Type]; !canParse {
			return decl
		}
	}
	decl.Value = p.parseExpression(LowestPrec)
	return decl
}

func (p *Parser) parseCustomTypeDecl() *ast.TypeExpr {
	decl := &ast.TypeExpr{
		Token:  p.currentToken,
		Fields: make([]*ast.FieldExpr, 0),
	}

	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		p.errors = append(p.errors, &diag.EmptyCustomType{
			Phase: diag.PhaseParse,
			Pos:   decl.Token.Pos,
		})
		return decl
	}

	for {
		if !p.peekTokenIs(token.IDENT) {
			p.addUnexpectedToken(p.peekToken, token.IDENT)
			break
		}
		p.nextToken()
		field := &ast.FieldExpr{
			Token: p.currentToken,
			Name:  &ast.IdentExpr{Token: p.currentToken},
		}

		if !p.peekTokenIs(token.COLON) {
			p.addUnexpectedToken(p.peekToken, token.COLON)
			break
		}
		p.nextToken()

		if !p.peekTokenIs(token.IDENT) {
			p.addUnexpectedToken(p.peekToken, token.IDENT)
			break
		}
		p.nextToken()
		field.Type = &ast.IdentExpr{Token: p.currentToken}

		decl.Fields = append(decl.Fields, field)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			continue
		}

		if p.peekTokenIs(token.RBRACE) {
			p.nextToken()
			break
		}

		p.addUnexpectedToken(p.peekToken, token.RBRACE)
		break
	}

	return decl
}

func (p *Parser) parseExpression(prec precedence) ast.Expression {
	prefFn, ok := p.prefixFns[p.currentToken.Type]
	if !ok {
		p.addExpressionExpected(p.currentToken)
		return &ast.BadExpr{Token: p.currentToken}
	}
	leftExpr := prefFn()
	for !p.peekTokenIs(token.NEWLINE) && prec < p.peekPrecedence() {
		infixExprFn, okInf := p.infixFns[p.peekToken.Type]
		if !okInf {
			return leftExpr
		}
		p.nextToken()
		leftExpr = infixExprFn(leftExpr)
	}
	return leftExpr
}

func (p *Parser) parsePrefixExpression() ast.Expression {
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

func (p *Parser) parseCallExpr(left ast.Expression) ast.Expression {
	expr := ast.MemberAccessExpr{Token: p.currentToken, Object: left}
	p.nextToken() // advance past `.` onto the member name
	if !p.currTokenIs(token.IDENT) {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
		return &ast.BadExpr{Token: p.currentToken}
	}
	expr.Method = p.parseIdentExpr()
	return &expr
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

func (p *Parser) parseBoolean() ast.Expression {
	val, _ := strconv.ParseBool(p.currentToken.Literal)
	return &ast.BooleanExpr{Token: p.currentToken, Value: val}
}

// Helpers

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

func (p *Parser) addExpressionExpected(tok token.Token) {
	p.errors = append(p.errors, &diag.ExpressionExpected{
		Phase: diag.PhaseParse,
		Pos:   tok.Pos,
		Got:   tok.Type,
	})
}

func (p *Parser) addUnexpectedToken(tok token.Token, expected token.TokenType) {
	p.errors = append(p.errors, &diag.UnexpectedToken{
		Phase:    diag.PhaseParse,
		Pos:      tok.Pos,
		Got:      tok.Type,
		Expected: expected,
	})
}

func (p *Parser) addNotImplemented(pos token.Pos, subject string) {
	p.errors = append(p.errors, diag.NotImplemented{
		Phase:   diag.PhaseParse,
		Pos:     pos,
		Subject: subject,
	})
}

func (p *Parser) addParseFailed(pos token.Pos, target token.TokenType, _ error) {
	p.errors = append(p.errors, &diag.ParseFailed{
		Phase:  diag.PhaseParse,
		Pos:    pos,
		Target: target,
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
