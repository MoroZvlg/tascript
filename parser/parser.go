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
		token.PERCENT:  p.parseInfixExpr,
		token.EQ:       p.parseInfixExpr,
		token.NEQ:      p.parseInfixExpr,
		token.LT:       p.parseInfixExpr,
		token.GT:       p.parseInfixExpr,
		token.LTEQ:     p.parseInfixExpr,
		token.GTEQ:     p.parseInfixExpr,
		token.AND:      p.parseInfixExpr,
		token.OR:       p.parseInfixExpr,
		token.DOT:      p.parseMemberExpr,
		token.LPAREN:   p.parseCallExpr,
		token.LBRACKET: p.parseIndexExpr,
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
			decl := p.parseOutputDecl()
			if len(p.errors) == errsBefore {
				prog.Outputs = append(prog.Outputs, decl)
			}
		case token.FUNCTION:
			decl := p.parseFunctionDecl()
			if len(p.errors) == errsBefore {
				switch decl.Identifier.String() {
				case InitFnIdent:
					prog.InitFn = decl
				case RunFnIdent:
					prog.RunFn = decl
				default: // Unreachable
				}
			}
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

	if prog.RunFn == nil && len(p.errors) == 0 {
		prog.Valid = false
		p.addMissingRun(p.currentToken.Pos)
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
		p.addTypeOrCustomTypeExpected(p.currentToken.Pos)
	}
	return decl
}

func (p *Parser) parseOutputDecl() *ast.OutputDecl {
	decl := &ast.OutputDecl{Token: p.currentToken}

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
		p.addTypeOrCustomTypeExpected(p.currentToken.Pos)
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

func (p *Parser) parseFunctionDecl() *ast.FunctionDecl {
	decl := &ast.FunctionDecl{Token: p.currentToken}
	p.nextToken()
	errsBeforeDecl := len(p.errors)
	if p.currTokenIs(token.IDENT) {
		if p.currentToken.Literal != InitFnIdent && p.currentToken.Literal != RunFnIdent {
			p.addForbiddenFunc(p.currentToken.Pos)
			// don't need to parse func body if it's forbidden func
			return decl
		}

		decl.Identifier = &ast.IdentExpr{Token: p.currentToken}
		p.nextToken()
	} else {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
	}

	if p.currTokenIs(token.LPAREN) {
		p.nextToken()
	} else {
		if len(p.errors) == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.LPAREN)
		}
	}
	if p.currTokenIs(token.RPAREN) {
		p.nextToken()
	} else {
		if len(p.errors) == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.RPAREN)
		}
	}

	if !p.currTokenIs(token.LBRACE) {
		if len(p.errors) == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.LBRACE)
		}
		// we didn't find func block declaration start. don't need to parse
		return decl
	}

	errsBeforeBody := len(p.errors)
	decl.Body = p.parseBlock() // leaves current ON `}` (or EOF)

	// only Run (and the unreachable unnamed case) reject a genuinely empty body
	notInitFn := decl.Identifier == nil || decl.Identifier.String() == RunFnIdent
	if len(decl.Body.Stmts) == 0 && len(p.errors) == errsBeforeBody && notInitFn {
		p.addEmptyFunctionBody(p.currentToken.Pos)
	}

	return decl
}

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

		errsBeforeStmt := len(p.errors)
		block.Stmts = append(block.Stmts, p.parseStatement())
		switch {
		case len(p.errors) != errsBeforeStmt:
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
	default:
		return p.parseExprOrAssignStmt()
	}
}

func (p *Parser) parseIfStmt() ast.Statement {
	ifTok := p.currentToken
	stmt := &ast.IfStmt{Token: ifTok}
	beforeCondErrs := len(p.errors)

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken() // current = `(`
		p.nextToken() // current = first condition token
		stmt.Condition = p.parseExpression(LowestPrec)
		if p.peekTokenIs(token.RPAREN) {
			p.nextToken() // current = `)`
		} else if len(p.errors) == beforeCondErrs {
			p.addUnexpectedToken(p.peekToken, token.RPAREN)
		}
	} else {
		// didn't find condition. but will try to parse blocks anyway
		p.addUnexpectedToken(p.peekToken, token.LPAREN)
	}

	if p.peekTokenIs(token.LBRACE) {
		p.nextToken() // current = `{`
	} else {
		if len(p.errors) == beforeCondErrs {
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

func (p *Parser) parseCustomTypeDecl() *ast.TypeExpr {
	decl := &ast.TypeExpr{
		Token:  p.currentToken,
		Fields: make([]*ast.FieldExpr, 0),
	}
	p.skipPeekNewLines()

	if p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		p.addEmptyCustomType(decl.Token.Pos)
		return decl
	}

	for {
		p.skipPeekNewLines()

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

		p.skipPeekNewLines()

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

func (p *Parser) parseMemberExpr(left ast.Expression) ast.Expression {
	expr := ast.MemberAccessExpr{Token: p.currentToken, Object: left}
	p.nextToken() // advance past `.` onto the member name
	if !p.currTokenIs(token.IDENT) {
		p.addUnexpectedToken(p.currentToken, token.IDENT)
		return &ast.BadExpr{Token: p.currentToken}
	}
	expr.Method = p.parseIdentExpr()
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

		errsBefore := len(p.errors)
		if p.currTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			kwargSeen = true
			kwarg := &ast.KwargsExpr{Token: p.currentToken, Key: &ast.IdentExpr{Token: p.currentToken}}
			p.nextToken() // consume `=`
			p.nextToken()
			value := p.parseExpression(LowestPrec)
			if len(p.errors) == errsBefore {
				kwarg.Value = value
				callExpr.Kwargs = append(callExpr.Kwargs, kwarg)
			}
		} else {
			if kwargSeen {
				p.addArgsOrder(p.currentToken.Pos)
				break
			}
			arg := p.parseExpression(LowestPrec)
			if len(p.errors) == errsBefore {
				callExpr.Args = append(callExpr.Args, arg)
			}
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
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

func (p *Parser) addTypeOrCustomTypeExpected(pos token.Pos) {
	p.errors = append(p.errors, &diag.TypeOrCustomTypeExpected{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addEmptyCustomType(pos token.Pos) {
	p.errors = append(p.errors, &diag.EmptyCustomType{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addForbiddenFunc(pos token.Pos) {
	p.errors = append(p.errors, &diag.ForbiddenFunc{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addEmptyFunctionBody(pos token.Pos) {
	p.errors = append(p.errors, &diag.EmptyFunctionBody{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addArgsOrder(pos token.Pos) {
	p.errors = append(p.errors, &diag.ArgsOrder{
		Phase: diag.PhaseParse,
		Pos:   pos,
	})
}

func (p *Parser) addMissingRun(pos token.Pos) {
	p.errors = append(p.errors, &diag.MissingRunFunc{
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
