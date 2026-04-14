package types

type EnumType struct {
	Name   string
	Values map[string]int32
}

func (et *EnumType) GetName() string {
	return et.Name
}

func (et *EnumType) IsEqual(t Type) bool {
	if other, ok := t.(*EnumType); ok {
		return et.Name == other.Name
	}
	return false
}

func (et *EnumType) HasMember(member string) (bool, Type) {
	if _, ok := et.Values[member]; ok {
		return true, Int32Type
	}
	return false, nil
}

func (et *EnumType) GetMembers() []Member {
	members := make([]Member, 0, len(et.Values))
	for name := range et.Values {
		members = append(members, Member{Name: name, Type: Int32Type})
	}
	return members
}
