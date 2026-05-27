// Package parser builds an [ast.Program] from a token stream.
package parser

import (
	"fmt"
	"strconv"

	"github.com/MoroZvlg/tascript/ast"
	"github.com/MoroZvlg/tascript/diag"
	"github.com/MoroZvlg/tascript/token"
)

type Parser struct {
	toks   []token.Token
	cursor int
	diags  []diag.Diagnostic
}

func New(toks []token.Token) *Parser { return &Parser{toks: toks} }

func (p *Parser) Diagnostics() []diag.Diagnostic { return p.diags }

func (p *Parser) Parse() *ast.Program {
	prog := &ast.Program{}
	for {
		p.skipNewlines()
		if p.peek().Kind == token.EOF {
			break
		}
		decl := p.parseTopDecl()
		if decl == nil {
			p.resyncToNewline()
			continue
		}
		prog.Decls = append(prog.Decls, decl)
		if fn, ok := decl.(*ast.FuncDecl); ok {
			switch fn.Name {
			case "Init":
				if prog.Init == nil {
					prog.Init = fn
				}
			case "Run":
				if prog.Run == nil {
					prog.Run = fn
				}
			}
		}
	}
	return prog
}

func (p *Parser) parseTopDecl() ast.TopDecl {
	t := p.peek()
	if t.Kind != token.IDENT {
		p.addErrf(t.Pos, diag.CatTopLevelForm,
			"expected top-level declaration, got %s", t)
		return nil
	}
	switch t.Literal {
	case "function":
		return p.parseFuncDecl()
	case "input":
		return p.parseInputDecl()
	case "output":
		return p.parseOutputDecl()
	default:
		return p.parseTopAssign()
	}
}

func (p *Parser) parseFuncDecl() *ast.FuncDecl {
	p.advance() // skip 'function'
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"expected function name after 'function', got %s", name)
		return nil
	}
	p.advance()

	if !p.expect(token.LPAREN, "after function name") {
		return nil
	}
	params, ok := p.parseParams()
	if !ok {
		return nil
	}
	if !p.expect(token.LBRACE, "function body") {
		return nil
	}
	body, _ := p.parseBlockBody()
	return &ast.FuncDecl{Name: name.Literal, NamePos: name.Pos, Params: params, Body: body}
}

func (p *Parser) parseParams() ([]ast.Param, bool) {
	var params []ast.Param
	p.skipNewlines()
	if p.peek().Kind == token.RPAREN {
		p.advance()
		return params, true
	}
	for {
		p.skipNewlines()
		name := p.peek()
		if name.Kind != token.IDENT {
			p.addErrf(name.Pos, diag.CatTopLevelForm,
				"expected parameter name, got %s", name)
			return params, false
		}
		p.advance()
		params = append(params, ast.Param{Name: name.Literal, NamePos: name.Pos})
		p.skipNewlines()
		switch p.peek().Kind {
		case token.COMMA:
			p.advance()
			continue
		case token.RPAREN:
			p.advance()
			return params, true
		default:
			p.addErrf(p.peek().Pos, diag.CatTopLevelForm,
				"expected ',' or ')' after parameter, got %s", p.peek())
			return params, false
		}
	}
}

func (p *Parser) parseBlockBody() ([]ast.Stmt, bool) {
	var body []ast.Stmt
	for {
		p.skipNewlines()
		switch p.peek().Kind {
		case token.RBRACE:
			p.advance()
			return body, true
		case token.EOF:
			p.addErrf(p.peek().Pos, diag.CatTopLevelForm,
				"unterminated block - expected '}'")
			return body, false
		}
		stmt := p.parseStmt()
		if stmt == nil {
			p.resyncToStmtBoundary()
			continue
		}
		body = append(body, stmt)
	}
}

func (p *Parser) parseStmt() ast.Stmt {
	t := p.peek()
	if t.Kind == token.IDENT && t.Literal == "if" {
		return p.parseIf()
	}
	if t.Kind == token.IDENT && t.Literal == "emit" {
		return p.parseEmit()
	}
	expr := p.parseExpr()
	if expr == nil {
		return nil
	}
	if p.peek().Kind == token.ASSIGN {
		p.advance()
		val := p.parseExpr()
		if val == nil {
			return nil
		}
		return &ast.AssignStmt{Target: expr, Value: val}
	}
	return &ast.ExprStmt{Expr: expr}
}

func (p *Parser) parseIf() *ast.IfStmt {
	ifT := p.consume()
	if !p.expect(token.LPAREN, "after 'if'") {
		return nil
	}
	p.skipNewlines()
	cond := p.parseExpr()
	if cond == nil {
		return nil
	}
	p.skipNewlines()
	if !p.expect(token.RPAREN, "after if condition") {
		return nil
	}
	if !p.expect(token.LBRACE, "if body") {
		return nil
	}
	cons, ok := p.parseBlockBody()
	if !ok {
		return nil
	}
	stmt := &ast.IfStmt{IfPos: ifT.Pos, Condition: cond, Consequence: cons}
	p.skipNewlines()
	if p.peek().Kind == token.IDENT && p.peek().Literal == "else" {
		p.advance()
		if !p.expect(token.LBRACE, "else body") {
			return stmt
		}
		alt, _ := p.parseBlockBody()
		stmt.Alternative = alt
	}
	return stmt
}

func (p *Parser) parseEmit() *ast.EmitStmt {
	emitT := p.consume()
	if !p.expect(token.LPAREN, "after 'emit'") {
		return nil
	}
	p.skipNewlines()
	namedOut := p.peek()
	if namedOut.Kind != token.IDENT {
		p.addErrf(namedOut.Pos, diag.CatTopLevelForm,
			"first emit() argument must be an output identifier, got %s", namedOut)
		return nil
	}
	p.advance()
	stmt := &ast.EmitStmt{CallPos: emitT.Pos, Output: namedOut.Literal, OutputPos: namedOut.Pos}
	for {
		p.skipNewlines()
		if p.peek().Kind != token.COMMA {
			break
		}
		p.advance()
		p.skipNewlines()
		if p.peek().Kind == token.IDENT && p.peekAt(1).Kind == token.ASSIGN {
			kwArg := p.parseKwarg()
			if kwArg == nil {
				return nil
			}
			stmt.Kwargs = append(stmt.Kwargs, *kwArg)
			continue
		}
		if stmt.Value != nil || len(stmt.Kwargs) > 0 {
			p.addErrf(p.peek().Pos, diag.CatEmitPayload,
				"emit() positional value must come before any keyword arguments")
			return nil
		}
		val := p.parseExpr()
		if val == nil {
			return nil
		}
		stmt.Value = val
	}
	p.skipNewlines()
	if !p.expect(token.RPAREN, "after emit(...) arguments") {
		return nil
	}
	return stmt
}

func (p *Parser) parseKwarg() *ast.KwArg {
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"keyword arguments require an identifier, got %s", name)
		return nil
	}
	p.advance()
	if !p.expect(token.ASSIGN, "after keyword argument name") {
		return nil
	}
	val := p.parseExpr()
	if val == nil {
		return nil
	}
	return &ast.KwArg{Name: name.Literal, NamePos: name.Pos, Value: val}
}

func (p *Parser) parseTopAssign() ast.TopDecl {
	name := p.consume()
	if !p.expect(token.ASSIGN, "after top-level name") {
		return nil
	}
	val := p.parseExpr()
	if val == nil {
		return nil
	}
	return &ast.ConstDecl{Name: name.Literal, NamePos: name.Pos, Value: val}
}

// parseInputDecl parses `input <name>: <Type>`.
func (p *Parser) parseInputDecl() ast.TopDecl {
	p.advance() // 'input'
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"expected input port name after 'input', got %s", name)
		return nil
	}
	p.advance()
	if !p.expect(token.COLON, "after input port name") {
		return nil
	}
	typ := p.peek()
	if typ.Kind != token.IDENT {
		p.addErrf(typ.Pos, diag.CatTopLevelForm,
			"expected input type after ':', got %s", typ)
		return nil
	}
	p.advance()
	return &ast.InputDecl{Name: name.Literal, NamePos: name.Pos, Type: typ.Literal, TypePos: typ.Pos}
}

// parseOutputDecl parses the three §3.3 output shapes:
//
//	output <name>: <ValueType>
//	output <name> { field: Type, ... }
//	output <name>: <ValueType> { field: Type, ... }
func (p *Parser) parseOutputDecl() ast.TopDecl {
	p.advance() // 'output'
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"expected output port name after 'output', got %s", name)
		return nil
	}
	p.advance()
	out := &ast.OutputDecl{Name: name.Literal, NamePos: name.Pos}

	if p.peek().Kind == token.COLON {
		p.advance()
		vt := p.peek()
		if vt.Kind != token.IDENT {
			p.addErrf(vt.Pos, diag.CatTopLevelForm,
				"expected output value type after ':', got %s", vt)
			return nil
		}
		p.advance()
		out.ValueType = vt.Literal
		out.ValueTypePos = vt.Pos
	}

	if p.peek().Kind == token.LBRACE {
		out.Structured = true
		p.advance()
		for {
			p.skipNewlines()
			switch p.peek().Kind {
			case token.RBRACE:
				p.advance()
				goto done
			case token.EOF:
				p.addErrf(p.peek().Pos, diag.CatTopLevelForm,
					"unterminated output schema - expected '}'")
				return out
			}
			f := p.parseSchemaField()
			if f == nil {
				p.resyncToFieldBoundary()
				continue
			}
			out.Fields = append(out.Fields, *f)
			if p.peek().Kind == token.COMMA {
				p.advance()
			}
		}
	}
done:
	if out.ValueType == "" && !out.Structured {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"output %q must declare a value type (: Type) and/or a { ... } schema", name.Literal)
	}
	return out
}

func (p *Parser) parseSchemaField() *ast.SchemaField {
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"expected schema field name, got %s", name)
		return nil
	}
	p.advance()
	if !p.expect(token.COLON, "after schema field name") {
		return nil
	}
	typ := p.peek()
	if typ.Kind != token.IDENT {
		p.addErrf(typ.Pos, diag.CatTopLevelForm,
			"expected field type after ':', got %s", typ)
		return nil
	}
	p.advance()
	return &ast.SchemaField{Name: name.Literal, NamePos: name.Pos, Type: typ.Literal, TypePos: typ.Pos}
}

type precedence int

const (
	precLowest precedence = iota
	precOr
	precAnd
	precEquality
	precCompare
	precSum
	precProduct
	precPrefix
	precPostfix
)

func (p *Parser) parseExpr() ast.Expr {
	return p.parseExprPrec(precLowest)
}

func (p *Parser) parseExprPrec(min precedence) ast.Expr {
	left := p.parsePrefixExpr()
	if left == nil {
		return nil
	}
	for {
		p.skipExprNewlines()
		switch p.peek().Kind {
		case token.LPAREN:
			if precPostfix < min {
				return left
			}
			left = p.parseCallExpr(left)
			if left == nil {
				return nil
			}
			continue
		case token.DOT:
			if precPostfix < min {
				return left
			}
			left = p.parseMemberExpr(left)
			if left == nil {
				return nil
			}
			continue
		case token.LBRACKET:
			if precPostfix < min {
				return left
			}
			left = p.parseIndexExpr(left)
			if left == nil {
				return nil
			}
			continue
		}

		opPrec, ok := infixPrecedence(p.peek().Kind)
		if !ok || opPrec < min {
			return left
		}
		op := p.consume()
		right := p.parseExprPrec(opPrec + 1)
		if right == nil {
			return nil
		}
		left = &ast.BinaryExpr{Left: left, Op: op.Kind, OpPos: op.Pos, Right: right}
	}
}

func (p *Parser) parsePrefixExpr() ast.Expr {
	p.skipNewlines()
	t := p.peek()
	switch t.Kind {
	case token.NUMBER:
		p.advance()
		v, err := strconv.ParseFloat(t.Literal, 64)
		if err != nil {
			p.addErrf(t.Pos, diag.CatTopLevelForm,
				"invalid number literal %q", t.Literal)
			return nil
		}
		return &ast.NumberLit{Val: v, P: t.Pos}
	case token.STRING:
		p.advance()
		return &ast.StringLit{Val: t.Literal, P: t.Pos}
	case token.IDENT:
		p.advance()
		switch t.Literal {
		case "true":
			return &ast.BoolLit{Val: true, P: t.Pos}
		case "false":
			return &ast.BoolLit{Val: false, P: t.Pos}
		default:
			return &ast.Ident{Name: t.Literal, P: t.Pos}
		}
	case token.BANG, token.MINUS:
		p.advance()
		right := p.parseExprPrec(precPrefix)
		if right == nil {
			return nil
		}
		return &ast.UnaryExpr{Op: t.Kind, OpPos: t.Pos, Right: right}
	case token.LPAREN:
		p.advance()
		p.skipNewlines()
		expr := p.parseExpr()
		if expr == nil {
			return nil
		}
		p.skipNewlines()
		if !p.expect(token.RPAREN, "after grouped expression") {
			return nil
		}
		return expr
	}
	p.addErrf(t.Pos, diag.CatTopLevelForm,
		"expected expression, got %s", t)
	return nil
}

func (p *Parser) parseCallExpr(callee ast.Expr) ast.Expr {
	lparen := p.consume()
	call := &ast.CallExpr{Callee: callee, LPos: lparen.Pos}
	p.skipNewlines()
	if p.peek().Kind == token.RPAREN {
		p.advance()
		return call
	}
	for {
		p.skipNewlines()
		if p.peek().Kind == token.IDENT && p.peekAt(1).Kind == token.ASSIGN {
			kw := p.parseKwarg()
			if kw == nil {
				return nil
			}
			call.Args = append(call.Args, ast.CallArg{Name: kw.Name, NamePos: kw.NamePos, Value: kw.Value})
		} else {
			val := p.parseExpr()
			if val == nil {
				return nil
			}
			call.Args = append(call.Args, ast.CallArg{Value: val})
		}
		p.skipNewlines()
		switch p.peek().Kind {
		case token.COMMA:
			p.advance()
			continue
		case token.RPAREN:
			p.advance()
			return call
		default:
			p.addErrf(p.peek().Pos, diag.CatTopLevelForm,
				"expected ',' or ')' after call argument, got %s", p.peek())
			return nil
		}
	}
}

func (p *Parser) parseMemberExpr(object ast.Expr) ast.Expr {
	p.advance() // '.'
	name := p.peek()
	if name.Kind != token.IDENT {
		p.addErrf(name.Pos, diag.CatTopLevelForm,
			"expected member name after '.', got %s", name)
		return nil
	}
	p.advance()
	return &ast.MemberExpr{Object: object, Name: name.Literal, NamePos: name.Pos}
}

func (p *Parser) parseIndexExpr(object ast.Expr) ast.Expr {
	lbracket := p.consume()
	p.skipNewlines()
	index := p.parseExpr()
	if index == nil {
		return nil
	}
	p.skipNewlines()
	if !p.expect(token.RBRACKET, "after index expression") {
		return nil
	}
	return &ast.IndexExpr{Object: object, Index: index, LPos: lbracket.Pos}
}

func infixPrecedence(k token.Kind) (precedence, bool) {
	switch k {
	case token.OR:
		return precOr, true
	case token.AND:
		return precAnd, true
	case token.EQ, token.NEQ:
		return precEquality, true
	case token.LT, token.LTE, token.GT, token.GTE:
		return precCompare, true
	case token.PLUS, token.MINUS:
		return precSum, true
	case token.ASTERISK, token.SLASH, token.PERCENT:
		return precProduct, true
	default:
		return precLowest, false
	}
}

// helpers

func (p *Parser) peek() token.Token {
	if p.cursor >= len(p.toks) {
		return token.Token{Kind: token.EOF}
	}
	return p.toks[p.cursor]
}

func (p *Parser) peekAt(n int) token.Token {
	i := p.cursor + n
	if i >= len(p.toks) {
		return token.Token{Kind: token.EOF}
	}
	return p.toks[i]
}

func (p *Parser) advance() token.Token {
	t := p.peek()
	if p.cursor < len(p.toks) {
		p.cursor++
	}
	return t
}

func (p *Parser) consume() token.Token {
	t := p.peek()
	p.advance()
	return t
}

func (p *Parser) expect(k token.Kind, ctx string) bool {
	t := p.peek()
	if t.Kind != k {
		p.addErrf(t.Pos, diag.CatTopLevelForm,
			"expected %s %s, got %s", k, ctx, t)
		return false
	}
	p.advance()
	return true
}

func (p *Parser) skipNewlines() {
	for p.peek().Kind == token.NEWLINE {
		p.advance()
	}
}

func (p *Parser) skipExprNewlines() {
	for p.peek().Kind == token.NEWLINE {
		switch p.peekAt(1).Kind {
		case token.DOT, token.LPAREN, token.LBRACKET,
			token.PLUS, token.MINUS, token.ASTERISK, token.SLASH, token.PERCENT,
			token.EQ, token.NEQ, token.LT, token.LTE, token.GT, token.GTE,
			token.AND, token.OR:
			p.advance()
		default:
			return
		}
	}
}

func (p *Parser) resyncToNewline() {
	for {
		t := p.peek()
		if t.Kind == token.NEWLINE || t.Kind == token.EOF {
			return
		}
		p.advance()
	}
}

func (p *Parser) resyncToStmtBoundary() {
	for {
		switch p.peek().Kind {
		case token.NEWLINE, token.RBRACE, token.EOF:
			return
		}
		p.advance()
	}
}

// resyncToFieldBoundary recovers from a malformed schema field by skipping
// to the next field separator without consuming the closing '}' or EOF.
func (p *Parser) resyncToFieldBoundary() {
	for {
		switch p.peek().Kind {
		case token.NEWLINE, token.COMMA, token.RBRACE, token.EOF:
			return
		}
		p.advance()
	}
}

func (p *Parser) addErrf(pos token.Pos, cat diag.Category, format string, args ...any) {
	p.diags = append(p.diags, diag.Diagnostic{
		Phase: diag.PhaseParse, Category: cat, Pos: pos, Msg: fmt.Sprintf(format, args...),
	})
}
