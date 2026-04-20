package parser

import (
	"fmt"
	"strconv"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

type Parser struct {
	l            *lexer.Lexer
	currentToken token.Token
	peekToken    token.Token
	errors       []error
	prefixFns    map[token.TokenType]func() ast.Expression
	infixFns     map[token.TokenType]func(ast.Expression) ast.Expression
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
	p.nextToken()
	p.nextToken()
	p.prefixFns = map[token.TokenType]func() ast.Expression{
		token.IDENT:  p.parseIdentifier,
		token.INT:    p.parseIntegerLiteral,
		token.MINUS:  p.parsePrefixExpression,
		token.BANG:   p.parsePrefixExpression,
		token.TRUE:   p.parseBoolean,
		token.FALSE:  p.parseBoolean,
		token.LPAREN: p.parseGroupedExpression,
		token.IF:     p.parseIfExpression,
	}
	p.infixFns = map[token.TokenType]func(ast.Expression) ast.Expression{
		token.PLUS:     p.parseInfixExpression,
		token.MINUS:    p.parseInfixExpression,
		token.ASTERISK: p.parseInfixExpression,
		token.SLASH:    p.parseInfixExpression,
		token.EQ:       p.parseInfixExpression,
		token.NEQ:      p.parseInfixExpression,
		token.LT:       p.parseInfixExpression,
		token.GT:       p.parseInfixExpression,
		token.LTEQ:     p.parseInfixExpression,
		token.GTEQ:     p.parseInfixExpression,
		token.AND:      p.parseInfixExpression,
		token.OR:       p.parseInfixExpression,
	}
	return p
}

func (p *Parser) Errors() []error {
	return p.errors
}

func (p *Parser) nextToken() {
	p.currentToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}

	for p.currentToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.currentToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.CONST:
		return p.parseConstStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.currentToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.currentToken, Value: p.currentToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken() // skip '='

	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken() // skip ';' if exists. optional
	}

	return stmt
}

func (p *Parser) parseConstStatement() *ast.ConstStatement {
	stmt := &ast.ConstStatement{Token: p.currentToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.currentToken, Value: p.currentToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}
	p.nextToken() // skip '='

	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken() // skip ';' if exists. optional
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.currentToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken() // skip ';' if exists. optional
	}
	return stmt
}

func (p *Parser) parseExpression(prec precedence) ast.Expression {
	prefixFn, ok := p.prefixFns[p.currentToken.Type]
	if !ok {
		p.errors = append(p.errors, fmt.Errorf("no prefix parse function for %s", p.currentToken.Type))
		return nil
	}
	leftExpr := prefixFn()
	for !p.peekTokenIs(token.SEMICOLON) && prec < p.peekPrecedence() {
		infix, okInf := p.infixFns[p.peekToken.Type]
		if !okInf {
			return leftExpr
		}
		p.nextToken()
		leftExpr = infix(leftExpr)
	}
	return leftExpr
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.currentToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken() // skip ';' if exists. optional
	}

	return stmt
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.currentToken, Value: p.currentToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	i, err := strconv.ParseInt(p.currentToken.Literal, 0, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Errorf("invalid integer literal: %s", p.currentToken.Literal))
		return nil
	}
	return &ast.IntegerLiteral{Token: p.currentToken, Value: i}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.currentToken, Value: p.currentToken.Type == token.TRUE}
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()                     // consume '('
	expr := p.parseExpression(LOWEST) // parse inside
	if !p.expectPeek(token.RPAREN) {  // must close
		return nil
	}
	return expr
}

func (p *Parser) parseIfExpression() ast.Expression {
	expr := &ast.IfExpression{
		Token: p.currentToken,
	}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	p.nextToken() // skip "("

	expr.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	expr.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()
		if !p.expectPeek(token.LBRACE) {
			return nil
		}
		expr.Alternative = p.parseBlockStatement()
	}

	return expr
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.currentToken}
	p.nextToken() // consume '{'
	for !p.currentTokenIs(token.RBRACE) && !p.currentTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expr := &ast.PrefixExpression{
		Token:    p.currentToken,
		Operator: p.currentToken.Literal,
	}
	p.nextToken()
	expr.Right = p.parseExpression(PREFIX)
	return expr
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expr := ast.InfixExpression{Token: p.currentToken, Operator: p.currentToken.Literal, Left: left}
	prec := p.currentPrecedence()
	p.nextToken()
	expr.Right = p.parseExpression(prec)
	return &expr
}

func (p *Parser) expectPeek(tokenType token.TokenType) bool {
	if p.peekTokenIs(tokenType) {
		p.nextToken()
		return true
	}
	p.errors = append(p.errors, fmt.Errorf("expected next token to be %s, got %s", tokenType, p.peekToken.Type))
	return false
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

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) currentTokenIs(t token.TokenType) bool {
	return p.currentToken.Type == t
}
