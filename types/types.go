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
	VoidType    = registerType(PrimitiveType{"void"})
)

type Type interface {
	GetName() string
	IsEqual(t Type) bool
}

type IndexedType interface {
	HasMember(member string) (bool, Type)
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
	t = LazyResolve(t)

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
	aType = LazyResolve(aType)

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

type SignatureType struct {
	ArgumentTypes []Type
	ReturnType    Type
	ArgumentNames []string
	Extern        bool
}

func (st *SignatureType) GetName() string {
	names := []string{}

	for _, arg := range st.ArgumentTypes {
		names = append(names, arg.GetName())
	}

	returnTypeStr := "void"

	if st.ReturnType != nil {
		returnTypeStr = st.ReturnType.GetName()
	}

	return fmt.Sprintf("(%s) -> %s", strings.Join(names, ", "), returnTypeStr)
}

func (st *SignatureType) IsEqual(aType Type) bool {
	aType = LazyResolve(aType)

	if st == aType {
		return true
	}

	if signType, ok := aType.(*SignatureType); ok {
		if st.ReturnType != nil {
			if signType.ReturnType == nil || !st.ReturnType.IsEqual(signType.ReturnType) {
				return false
			}
		} else if signType.ReturnType != nil {
			return false
		}

		thisTypes := st.ArgumentTypes
		aTypes := signType.ArgumentTypes

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

type ArrayType struct {
	Type   Type
	Length int64
}

func (at *ArrayType) GetName() string {
	if at.Length > -1 {
		return fmt.Sprintf("[%d]%s", at.Length, at.Type.GetName())
	}
	return fmt.Sprintf("[]%s", at.Type.GetName())
}

func (at *ArrayType) IsEqual(aType Type) bool {
	aType = LazyResolve(aType)

	if at == aType {
		return true
	}

	if arrayType, ok := aType.(*ArrayType); ok {
		if !at.Type.IsEqual(arrayType.Type) {
			return false
		}

		if at.Length > -1 && arrayType.Length > -1 {
			return at.Length == arrayType.Length
		}

		return true
	}

	return false
}

type StructType struct {
	Name      string
	Variables []struct {
		Name string
		Type Type
	}
	Functions []struct {
		Name string
		Type *SignatureType
	}
}

func (st *StructType) GetName() string {
	var name = "struct"
	if st.Name != "" {
		name = name + " " + st.Name
	}

	names := []string{}

	for _, v := range st.Variables {
		names = append(names, v.Name+": "+v.Type.GetName())
	}

	return fmt.Sprintf("%s { %s }", name, strings.Join(names, ", "))
}

func (st *StructType) IsEqual(aType Type) bool {
	aType = LazyResolve(aType)

	if st == aType {
		return true
	}

	if structType, ok := aType.(*StructType); ok {
		if len(st.Variables) != len(structType.Variables) {
			return false
		} else if len(st.Functions) != len(structType.Functions) {
			return false
		}

		for i, v := range st.Variables {
			if !v.Type.IsEqual(structType.Variables[i].Type) {
				return false
			}
		}

		for i, v := range st.Functions {
			if !v.Type.IsEqual(structType.Functions[i].Type) {
				return false
			}
		}

		return true
	}

	return false
}

func (st *StructType) HasMember(member string) (bool, Type) {
	for _, v := range st.Variables {
		if v.Name == member {
			return true, v.Type
		}
	}

	for _, v := range st.Functions {
		if v.Name == member {
			return true, v.Type
		}
	}

	return false, nil
}

type LazyType struct {
	Resolver func() Type
}

func (lt LazyType) GetName() string {
	return lt.Resolver().GetName()
}

func (lt LazyType) IsEqual(t Type) bool {
	return lt.Resolver().IsEqual(t)
}

func (lt LazyType) HasMember(member string) (bool, Type) {
	typ := lt.Resolver()
	if itype, ok := typ.(IndexedType); ok {
		return itype.HasMember(member)
	}
	return false, nil
}

func LazyResolve(t Type) Type {
	if t, ok := t.(*LazyType); ok {
		return t.Resolver()
	}
	return t
}

func registerType(typ Type) Type {
	Types[typ.GetName()] = typ
	return typ
}
