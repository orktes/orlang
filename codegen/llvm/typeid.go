package llvm

// getTypeID returns the type ID for a given type name, assigning a new ID if needed
func (lcg *LLVMCodeGen) getTypeID(typeName string) int32 {
	if id, ok := lcg.typeIDs[typeName]; ok {
		return id
	}
	id := lcg.nextTypeID
	lcg.typeIDs[typeName] = id
	lcg.nextTypeID++
	return id
}
