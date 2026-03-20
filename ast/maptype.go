package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

type MapType struct {
	MapKeyword   scanner.Token
	LeftBracket  scanner.Token
	RightBracket scanner.Token
	KeyType      Type
	ValueType    Type
}

func (MapType) typeNode() {}

func (mt *MapType) StartPos() Position {
	return StartPositionFromToken(mt.MapKeyword)
}

func (mt *MapType) EndPos() Position {
	return mt.ValueType.EndPos()
}

func (mt *MapType) String() string {
	return fmt.Sprintf("map[%s]%s", mt.KeyType, mt.ValueType)
}
