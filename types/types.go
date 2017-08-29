package types

import (
	"fmt"

	"strings"
)

var Types map[string]Type = map[string]Type{}

var (
	Float64Type = registerType(PrimitiveType{"float64"})
	Int64Type   = registerType(PrimitiveType{"int64"})
	Float32Type = registerType(PrimitiveType{"float32"})
	Int32Type   = registerType(PrimitiveType{"int32"})
	StringType  = registerType(PrimitiveType{"string"})
	BoolType    = registerType(PrimitiveType{"bool"})
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

type TupleType struct {
	Types []Type
}

func (tt *TupleType) GetName() string {
	names := []string{}

	for _, typ := range tt.Types {
		names = append(names, typ.GetName())
	}

	return fmt.Sprintf("(%s)", strings.Join(names, ", "))
}

func (tt *TupleType) IsEqual(aType Type) bool {
	if tt == aType {
		return true
	}
	if tuppleType, ok := aType.(*TupleType); ok {
		aTypes := tuppleType.Types
		thisTypes := tt.Types

		if len(aTypes) != len(thisTypes) {
			return false
		}

		for i, typ := range thisTypes {
			if !typ.IsEqual(aTypes[i]) {
				return false
			}
		}

		return true
	}
	return false
}

func registerType(typ Type) Type {
	Types[typ.GetName()] = typ
	return typ
}
