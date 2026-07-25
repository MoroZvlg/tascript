package parser

import (
	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/lexer"
	"github.com/MoroZvlg/tascript/token"
)

const maxParseErrors = 100

const (
	InitFnIdent     = "Init"
	RunFnIdent      = "Run"
	maxNestingDepth = 64
)

type Parser struct {
	l            *lexer.Lexer
	currentToken token.Token
	peekToken    token.Token
	errors       []diag.Diagnostic
	// slice may stop growing, so we add separate counter to replace len(errors)
	errCount int
	depth    int

	prefixFns map[token.TokenType]func() ast.Expression
	infixFns  map[token.TokenType]func(ast.Expression) ast.Expression
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
	p.nextToken()
	p.nextToken()

	p.prefixFns = map[token.TokenType]func() ast.Expression{
		token.IDENT:   p.parseIdentExpr,
		token.STATE:   p.parseIdentExpr,
		token.INTEGER: p.parseIntegerExpr,
		token.FLOAT:   p.parseFloatExpr,
		token.STRING:  p.parseStringExpr,
		token.MINUS:   p.parsePrefixExpr,
		token.BANG:    p.parsePrefixExpr,
		token.TRUE:    p.parseBoolExpr,
		token.FALSE:   p.parseBoolExpr,
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

func (p *Parser) addDiag(d diag.Diagnostic) {
	p.errCount++
	if len(p.errors) >= maxParseErrors {
		return
	}
	p.errors = append(p.errors, d)
}

func (p *Parser) Parse() *ast.Program {
	prog := &ast.Program{Valid: true}
	for !p.currTokenIs(token.EOF) {
		if p.currentToken.Type == token.NEWLINE {
			p.nextToken()
			continue
		}

		errsBefore := p.errCount
		switch p.currentToken.Type {
		case token.CONST:
			decl := p.parseConstDecl()
			if p.errCount == errsBefore {
				prog.Consts = append(prog.Consts, decl)
			}
		case token.INPUT:
			decl := p.parseInputDecl()
			if p.errCount == errsBefore {
				prog.Inputs = append(prog.Inputs, decl)
			}
		case token.OUTPUT:
			decl := p.parseOutputDecl()
			if p.errCount == errsBefore {
				prog.Outputs = append(prog.Outputs, decl)
			}
		case token.STATE:
			decl := p.parseStateFieldDecl()
			if p.errCount == errsBefore {
				prog.StateFields = append(prog.StateFields, decl)
			}
		case token.FUNCTION:
			decl := p.parseFunctionDecl()
			if p.errCount == errsBefore {
				switch decl.Identifier.String() {
				case InitFnIdent:
					if prog.InitFn != nil {
						// NOTE: this is incorrect phase for such an error. But otherwise we will replace func and that's it
						// Will see in the future if there will be a better way to handle it
						p.addDuplicateDecl(decl.Token, decl.Identifier.Token)
					} else {
						prog.InitFn = decl
					}
				case RunFnIdent:
					if prog.RunFn != nil {
						// NOTE: this is incorrect phase for such an error. But otherwise we will replace func and that's it
						// Will see in the future if there will be a better way to handle it
						p.addDuplicateDecl(decl.Token, decl.Identifier.Token)
					} else {
						prog.RunFn = decl
					}
				default: // Unreachable
				}
			}
		default:
			p.addUnexpectedTopDecl(p.currentToken.Pos)
		}
		if p.errCount == errsBefore &&
			!p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.EOF) {
			p.addUnexpectedToken(p.peekToken, token.NEWLINE)
		}
		if p.errCount > errsBefore {
			prog.Valid = false
			// no point of further parsing. we won't add diags anyway
			if p.errCount >= maxParseErrors {
				break
			}
			p.syncToNewLine()
		}
		p.nextToken()
	}

	if prog.RunFn == nil && p.errCount == 0 {
		prog.Valid = false
		p.addMissingRunFn(p.currentToken.Pos)
	}

	return prog
}

type portDecl struct {
	Token      token.Token
	Identifier *ast.IdentExpr
	Type       ast.TypeDecl
}

func (p *Parser) parsePortDecl() portDecl {
	decl := portDecl{Token: p.currentToken}

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
		// don't need to add second error on the same pos (we didn't move in case of missing IDENT)
		if decl.Identifier != nil {
			p.addUnexpectedToken(p.currentToken, token.COLON)
		}
		return decl
	}

	switch p.currentToken.Type {
	case token.IDENT:
		decl.Type = &ast.IdentExpr{Token: p.currentToken}
	case token.LBRACE:
		decl.Type = p.parseInlineTypeExpr()
	default:
		p.addTypeOrCustomTypeExpected(p.currentToken.Pos)
	}
	return decl
}

func (p *Parser) parseInputDecl() *ast.InputDecl {
	decl := p.parsePortDecl()
	return &ast.InputDecl{Token: decl.Token, Identifier: decl.Identifier, Type: decl.Type}
}

func (p *Parser) parseOutputDecl() *ast.OutputDecl {
	decl := p.parsePortDecl()
	return &ast.OutputDecl{Token: decl.Token, Identifier: decl.Identifier, Type: decl.Type}
}

func (p *Parser) parseStateFieldDecl() *ast.StateFieldDecl {
	decl := &ast.StateFieldDecl{Token: p.currentToken}

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
		// don't need to add second error on the same pos (we didn't move in case of missing IDENT)
		if decl.Identifier != nil {
			p.addUnexpectedToken(p.currentToken, token.COLON)
		}
		return decl
	}

	switch p.currentToken.Type {
	case token.IDENT:
		decl.Type = &ast.IdentExpr{Token: p.currentToken}
	// NOTE: subject of future improvements
	//case token.LBRACE:
	//	decl.Type = p.parseInlineTypeExpr()
	default:
		p.addTypeOrCustomTypeExpected(p.currentToken.Pos)
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken()
		p.nextToken()
		decl.Value = p.parseExpression(LowestPrec)
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
		// don't need to add second error on the same pos (we didn't move in case of missing IDENT)
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
	errsBeforeDecl := p.errCount
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
		if p.errCount == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.LPAREN)
		}
	}
	if p.currTokenIs(token.RPAREN) {
		p.nextToken()
	} else {
		if p.errCount == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.RPAREN)
		}
	}

	if !p.currTokenIs(token.LBRACE) {
		if p.errCount == errsBeforeDecl {
			p.addUnexpectedToken(p.currentToken, token.LBRACE)
		}
		// we didn't find func block declaration start. don't need to parse
		return decl
	}

	errsBeforeBody := p.errCount
	decl.Body = p.parseBlock() // leaves current ON `}` (or EOF)

	// only Run (and the unreachable unnamed case) reject a genuinely empty body
	notInitFn := decl.Identifier == nil || decl.Identifier.String() == RunFnIdent
	if len(decl.Body.Stmts) == 0 && p.errCount == errsBeforeBody && notInitFn {
		p.addEmptyFunctionBody(p.currentToken.Pos)
	}

	return decl
}

func (p *Parser) parseInlineTypeExpr() *ast.TypeExpr {
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
