package types

import "fmt"

var (
	Float64Type = PrimitiveType{"float64"}
	Int64Type   = PrimitiveType{"int64"}
	Float32Type = PrimitiveType{"float32"}
	Int32Type   = PrimitiveType{"int32"}
	StringType  = PrimitiveType{"string"}
)

type Type interface {
	GetName() string
	IsEqual(t Type) bool
}

type EntendedType interface {
	Entends(t Type) bool
}

type UnknownType string

func (ut UnknownType) GetName() string {
	return fmt.Sprintf("unknown (%s)", ut)
}

func (ut UnknownType) IsEqual(t Type) bool {
	return false
}

type PrimitiveType struct {
	Type string
}

func (pt PrimitiveType) GetName() string {
	return string(pt.Type)
}

func (pt PrimitiveType) IsEqual(t Type) bool {
	if typ, ok := t.(PrimitiveType); ok {
		return pt.Type == typ.Type
	} else if typ, ok := t.(EntendedType); ok {
		return typ.Entends(pt)
	}

	return false
}
