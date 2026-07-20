package analyser

import (
	"github.com/orktes/orlang/types"
)

// cTypeToOrlangType maps a canonicalized C type (from package cheader) to
// the closest orlang type for analysis purposes.
func cTypeToOrlangType(cType string) types.Type {
	if cType == "char*" {
		return types.StringType
	}
	if len(cType) > 0 && cType[len(cType)-1] == '*' {
		return &types.PointerType{Type: types.Int8Type}
	}

	switch cType {
	case "int":
		return types.Int32Type
	case "unsigned", "unsigned int":
		return types.UInt32Type
	case "long", "int64_t":
		return types.Int64Type
	case "unsigned long", "uint64_t", "size_t":
		return types.UInt64Type
	case "short", "int16_t":
		return types.Int16Type
	case "unsigned short", "uint16_t":
		return types.UInt16Type
	case "char", "int8_t":
		return types.Int8Type
	case "unsigned char", "uint8_t":
		return types.UInt8Type
	case "void":
		return types.VoidType
	case "float":
		return types.Float32Type
	case "double":
		return types.Float64Type
	default:
		return types.Int32Type
	}
}
