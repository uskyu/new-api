package dbx

import (
	"fmt"
)

func CoalesceExpr(expr, fallback string) string {
	return fmt.Sprintf("COALESCE(%s, %s)", expr, fallback)
}

func MoneyToCentsExpr(expr string) string {
	return fmt.Sprintf("ROUND(%s * 100, 0)", expr)
}

func CastInt64Expr(expr string) string {
	switch DetectDialect() {
	case DialectMySQL:
		return fmt.Sprintf("CAST(%s AS SIGNED)", expr)
	case DialectPostgres:
		return fmt.Sprintf("CAST(%s AS BIGINT)", expr)
	default:
		return fmt.Sprintf("CAST(%s AS INTEGER)", expr)
	}
}

func SumMoneyCentsExpr(expr string) string {
	sumExpr := fmt.Sprintf("SUM(%s)", CastInt64Expr(MoneyToCentsExpr(expr)))
	return CoalesceExpr(sumExpr, "0")
}
