package resolver_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/MoroZvlg/tascript/resolved"
)

func dumpConst(t *testing.T, resolvedConst *resolved.ConstDecl) string {
	t.Helper()
	return fmt.Sprintf(
		"const %s:%s = %s",
		resolvedConst.Name,
		resolvedConst.T,
		dump(t, resolvedConst.Value),
	)
}

func dump(t *testing.T, resolvedExpr resolved.Expression) string {
	t.Helper()
	switch expr := resolvedExpr.(type) {
	case *resolved.IntegerExpr:
		return fmt.Sprint(expr)
	case *resolved.FloatExpr:
		return fmt.Sprint(expr)
	case *resolved.StringExpr:
		return fmt.Sprint(expr)
	case *resolved.BooleanExpr:
		return fmt.Sprint(expr)
	case *resolved.IdentExpr:
		return fmt.Sprintf("%s:%s", expr, expr.Type())
	case *resolved.InfixExpr:
		return fmt.Sprintf("(infix:%s, %s, %s, %s)", expr.Type(), expr.Token.Literal, dump(t, expr.Left), dump(t, expr.Right))
	case *resolved.LogicalExpr:
		return fmt.Sprintf("(logical:%s, %s, %s, %s)", expr.Type(), expr.Token.Literal, dump(t, expr.Left), dump(t, expr.Right))
	case *resolved.PrefixExpr:
		return fmt.Sprintf("(prefix:%s, %s, %s)", expr.Type(), expr.Token.Literal, dump(t, expr.Right))
	case *resolved.CoerceExpr:
		return fmt.Sprintf("(coerce:%s, %s)", expr.Type(), dump(t, expr.Inner))
	case *resolved.MemberAccessExpr:
		return fmt.Sprintf("(member_access:%s, %s, %s)", expr.Type(), dump(t, expr.Object), expr.Method)
	case *resolved.MethodCallExpr:
		return fmt.Sprintf("(method_call:%s, %s, %s, %s)", expr.Type(), dump(t, expr.Receiver), expr.Method, dumpArgs(t, expr.Args))
	case *resolved.IndexExpr:
		return fmt.Sprintf("(index:%s, %s, %s)", expr.Type(), dump(t, expr.Left), dump(t, expr.Index))
	case *resolved.BadExpr:
		return "<bad expression>"
	default:
		return fmt.Sprint("<dump error>")
	}
}

func dumpStmt(t *testing.T, resolvedStmt resolved.Statement) string {
	t.Helper()
	switch stmt := resolvedStmt.(type) {
	case *resolved.LetStmt:
		return fmt.Sprintf("let %s:%s = %s", stmt.Name, stmt.T, dump(t, stmt.Value))
	case *resolved.AssignNameStmt:
		return fmt.Sprintf("%s:%s = %s", stmt.Target, stmt.T, dump(t, stmt.Value))
	case *resolved.ExprStmt:
		return dump(t, stmt.Expr)
	case *resolved.BlockStmt:
		return dumpBlock(t, stmt)
	case *resolved.IfStmt:
		var out bytes.Buffer
		out.WriteString("if ")
		out.WriteString(dump(t, stmt.Condition))
		out.WriteString(" ")
		out.WriteString(dumpBlock(t, stmt.Consequence))
		if stmt.Else != nil {
			out.WriteString(" else ")
			out.WriteString(dumpStmt(t, stmt.Else))
		}
		return out.String()
	case *resolved.EmitStmt:
		return fmt.Sprintf("emit(%s:%s, %s)", stmt.Output, stmt.T, dumpArgs(t, stmt.Args))
	case *resolved.BadStmt:
		return "<bad statement>"
	default:
		return fmt.Sprint("<dump error>")
	}
}

func dumpBlock(t *testing.T, block *resolved.BlockStmt) string {
	t.Helper()
	stmts := make([]string, 0, len(block.Stmts))
	for _, stmt := range block.Stmts {
		stmts = append(stmts, dumpStmt(t, stmt))
	}
	return "{" + strings.Join(stmts, "; ") + "}"
}

func dumpArgs(t *testing.T, args []*resolved.CallArgExpr) string {
	t.Helper()
	var out bytes.Buffer
	for i, arg := range args {
		out.WriteString(arg.Name)
		out.WriteString("=")
		out.WriteString(dump(t, arg.Value))
		if i < len(args)-1 {
			out.WriteString(",")
		}
	}
	return out.String()
}
