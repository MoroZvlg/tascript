package parser

type precedence int

const (
	LOWEST precedence = iota
	OR
	AND
	EQUALS
	COMPARE
	SUM
	PRODUCT
	PREFIX
	CALL
)
