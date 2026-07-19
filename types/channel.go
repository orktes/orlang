package types

import "fmt"

// ChannelType is a CSP channel carrying values of Elem. A nil Elem marks
// the untyped channel produced by the channel() builtin before it adopts
// the element type from its assignment context.
type ChannelType struct {
	Elem Type
}

func (ct *ChannelType) GetName() string {
	if ct.Elem == nil {
		return "chan"
	}
	return fmt.Sprintf("chan %s", ct.Elem.GetName())
}

func (ct *ChannelType) IsEqual(t Type) bool {
	if other, ok := t.(*ChannelType); ok {
		if ct.Elem == nil || other.Elem == nil {
			// Untyped channels adapt to any channel type
			return true
		}
		return ct.Elem.IsEqual(other.Elem)
	}
	return false
}
