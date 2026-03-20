package types

import "fmt"

type MapType struct {
	KeyType   Type
	ValueType Type
}

func (mt *MapType) GetName() string {
	return fmt.Sprintf("map[%s]%s", mt.KeyType.GetName(), mt.ValueType.GetName())
}

func (mt *MapType) IsEqual(t Type) bool {
	if other, ok := t.(*MapType); ok {
		return mt.KeyType.IsEqual(other.KeyType) && mt.ValueType.IsEqual(other.ValueType)
	}
	return false
}
