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
	errorMode    bool
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
		token.DOT:      p.praseCallExpr,
	}
	return p
}

func (p *Parser) Diagnostics() []diag.Diagnostic {
	return p.errors
}

func (p *Parser) Parse() *ast.Program {
	prog := &ast.Program{Valid: true}
	for p.currentToken.Type != token.EOF {
		if p.currentToken.Type == token.NEWLINE {
			p.nextToken()
		}

		switch p.currentToken.Type {
		case token.CONST:
			decl := p.parseConstDecl()
			if decl != nil {
				prog.Consts = append(prog.Consts, decl)
			}
			if p.errorMode {
				prog.Valid = false
				p.syncToNewLine()
				p.errorMode = false
			}
		case token.INPUT:
			p.addNotImplemented(p.currentToken.Pos, "inputDecl")
		case token.OUTPUT:
			p.addNotImplemented(p.currentToken.Pos, "outputDecl")
		case token.FUNCTION:
			p.addNotImplemented(p.currentToken.Pos, "functionDecl")
		default:
			p.addNotImplemented(p.currentToken.Pos, "unknownDecl. Add normal error!")
		}
		p.nextToken()
	}
	return prog
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

	assignPresent := true
	if p.currTokenIs(token.ASSIGN) {
		p.nextToken()
	} else {
		if decl.Identifier != nil {
			// don't need to add second error on the same pos(we didn't move in case of missing IDEN)
			p.addUnexpectedToken(p.currentToken, token.ASSIGN)
		}
		assignPresent = false
	}

	exprStartToken := p.currentToken
	errorsBeforeExpr := len(p.errors)
	decl.Value = p.parseExpression(LowestPrec)
	if decl.Value == nil && assignPresent && len(p.errors) == errorsBeforeExpr {
		p.addExpressionExpected(exprStartToken)
	}
	return decl
}

func (p *Parser) parseExpression(prec precedence) ast.Expression {
	prefFn, ok := p.prefixFns[p.currentToken.Type]
	if !ok {
		return nil
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
	p.nextToken()
	expr.Right = p.parseExpression(PrefixPrec)
	return expr
}

func (p *Parser) parseInfixExpr(left ast.Expression) ast.Expression {
	expr := ast.InfixExpr{Token: p.currentToken, Left: left}
	prec := p.currentPrecedence()
	p.nextToken()
	exprStartToken := p.currentToken
	expr.Right = p.parseExpression(prec)
	if expr.Right == nil {
		p.addExpressionExpected(exprStartToken)
	}
	return &expr
}

func (p *Parser) parseGroupExpr() ast.Expression {
	p.nextToken() // skip `(`
	expr := p.parseExpression(LowestPrec)
	if expr == nil || !p.peekTokenIs(token.RPAREN) {
		return nil
	}
	p.nextToken() // skip `)`

	return expr
}

func (p *Parser) praseCallExpr(left ast.Expression) ast.Expression {
	expr := ast.CallExpr{Token: p.currentToken, Object: left}

	if !p.peekTokenIs(token.IDENT) {
		return nil
	}
	p.nextToken()
	expr.Method = p.parseIdentExpr()
	return &expr
}

func (p *Parser) parseIdentExpr() ast.Expression {
	return &ast.IdentExpr{Token: p.currentToken}
}

func (p *Parser) parseIntegerExpr() ast.Expression {
	val, err := strconv.Atoi(p.currentToken.Literal)
	if err != nil {
		return nil
	}
	return &ast.IntegerExpr{Token: p.currentToken, Value: val}
}

func (p *Parser) parseFloatExpr() ast.Expression {
	val, err := strconv.ParseFloat(p.currentToken.Literal, 64)
	if err != nil {
		return nil
	}
	return &ast.FloatExpr{Token: p.currentToken, Value: val}
}

func (p *Parser) parseStringExpr() ast.Expression {
	return &ast.StringExpr{Token: p.currentToken, Value: p.currentToken.Literal}
}

func (p *Parser) parseBoolean() ast.Expression {
	val, err := strconv.ParseBool(p.currentToken.Literal)
	if err != nil {
		return nil
	}
	return &ast.BooleanExpr{Token: p.currentToken, Value: val}
}

// Helpers

func (p *Parser) syncToNewLine() {
	for {
		if p.currTokenIs(token.NEWLINE) {
			p.nextToken()
			break
		}
		if p.peekTokenIs(token.EOF) {
			break
		}
		p.nextToken()
	}
}

func (p *Parser) addExpressionExpected(tok token.Token) {
	p.errorMode = true

	p.errors = append(p.errors, &diag.ExpressionExpected{
		Phase: diag.PhaseParse,
		Pos:   tok.Pos,
		Got:   tok.Type,
	})
}

func (p *Parser) addUnexpectedToken(tok token.Token, expected token.TokenType) {
	p.errorMode = true

	p.errors = append(p.errors, &diag.UnexpectedToken{
		Phase:    diag.PhaseParse,
		Pos:      tok.Pos,
		Got:      tok.Type,
		Expected: expected,
	})
}

func (p *Parser) addNotImplemented(pos token.Pos, subject string) {
	p.errorMode = true

	p.errors = append(p.errors, diag.NotImplemented{
		Phase:   diag.PhaseParse,
		Pos:     pos,
		Subject: subject,
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
