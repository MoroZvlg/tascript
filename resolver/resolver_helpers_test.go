package resolver_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/MoroZvlg/tascript/resolved"
)

func dumpConst(t *testing.T, resolvedConst *resolved.ConstDecl) string {
	t.Helper()
	return fmt.Sprintf(
		"const %s:%s = %s",
		resolvedConst.Name,
		resolvedConst.Type(),
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
	case *resolved.PrefixExpr:
		return fmt.Sprintf("(prefix:%s, %s, %s)", expr.Type(), expr.Token.Literal, dump(t, expr.Right))
	case *resolved.CoerceExpr:
		return fmt.Sprintf("(coerce:%s, %s)", expr.Type(), dump(t, expr.Inner))
	case *resolved.MemberAccessExpr:
		return fmt.Sprintf("(member_access:%s, %s, %s)", expr.Type(), dump(t, expr.Object), expr.Method)
	case *resolved.MethodCallExpr:
		return fmt.Sprintf("(method_call:%s, %s, %s, %s)", expr.Type(), dump(t, expr.Receiver), expr.Method, dumpArgs(t, expr.Args))
	default:
		return fmt.Sprint("<dump error>")
	}
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
