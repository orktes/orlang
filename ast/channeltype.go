package ast

// ChannelType is the chan T type syntax.
type ChannelType struct {
	Start Position
	Type  Type // element type
}

func (c *ChannelType) StartPos() Position {
	return c.Start
}

func (c *ChannelType) EndPos() Position {
	return c.Type.EndPos()
}

func (_ *ChannelType) typeNode() {}
