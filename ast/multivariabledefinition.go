package ast

type MultiVariableDeclaration struct {
	Start        Position
	End          Position
	Declarations []*VariableDeclaration
}

func (vd *MultiVariableDeclaration) StartPos() Position {
	return vd.Start
}

func (vd *MultiVariableDeclaration) EndPos() Position {
	return vd.End
}

func (_ *MultiVariableDeclaration) stmtNode() {}
