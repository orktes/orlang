package llvm

// ValueContext indicates whether a value or address is needed when evaluating an expression
type ValueContext int

const (
	// ValueNeeded indicates the expression should be evaluated to a value (load from pointer if needed)
	ValueNeeded ValueContext = iota
	// AddressNeeded indicates the expression should be evaluated to an address (keep as pointer)
	AddressNeeded
)

// String returns a human-readable representation of the ValueContext
func (vc ValueContext) String() string {
	switch vc {
	case ValueNeeded:
		return "ValueNeeded"
	case AddressNeeded:
		return "AddressNeeded"
	default:
		return "Unknown"
	}
}
