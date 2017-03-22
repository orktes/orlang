package ast

type ComparisonExpressionOperator int

const (
	ComparisonExpressionOperatorEqual = ComparisonExpressionOperator(iota)
	ComparisonExpressionOperatorNotEqual
	ComparisonExpressionOperatorGreater
	ComparisonExpressionOperatorLess
	ComparisonExpressionOperatorGreaterOrEqual
	ComparisonExpressionOperatorLessOrEqual
)

type ComparisonExpression struct {
	Operator ComparisonExpressionOperator
	Left     Expression
	Right    Expression
}

func (a *ComparisonExpression) StartPos() Position {
	return a.Left.StartPos()
}

func (a *ComparisonExpression) EndPos() Position {
	return a.Right.EndPos()
}

func (_ *ComparisonExpression) exprNode() {}
