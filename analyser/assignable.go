package analyser

import (
	"fmt"
	"math"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
	"github.com/orktes/orlang/types"
)

// numericKind describes an orlang numeric primitive for implicit
// conversion purposes.
type numericKind struct {
	bits     int
	unsigned bool
	float    bool
}

var numericKinds = map[string]numericKind{
	"int8":    {8, false, false},
	"int16":   {16, false, false},
	"int32":   {32, false, false},
	"int64":   {64, false, false},
	"uint8":   {8, true, false},
	"uint16":  {16, true, false},
	"uint32":  {32, true, false},
	"uint64":  {64, true, false},
	"float32": {32, false, true},
	"float64": {64, false, true},
}

// isSafeNumericWidening reports whether a value of type src can implicitly
// convert to dst without losing information: widening int conversions that
// preserve the value, float32 to float64, and int to float.
func isSafeNumericWidening(src, dst types.Type) bool {
	if src == nil || dst == nil {
		return false
	}
	s, okS := numericKinds[src.GetName()]
	d, okD := numericKinds[dst.GetName()]
	if !okS || !okD {
		return false
	}

	switch {
	case s.float && d.float:
		return s.bits < d.bits
	case !s.float && d.float:
		return true
	case s.float && !d.float:
		return false
	default: // int -> int
		if s.unsigned == d.unsigned {
			return s.bits < d.bits
		}
		// unsigned -> larger signed always fits; signed -> unsigned never
		// implicitly (negative values would wrap).
		return s.unsigned && s.bits < d.bits
	}
}

// unwrapNumericLiteral looks through parens and a leading +/- for an int or
// float literal. Returns the literal and whether it was negated.
func unwrapNumericLiteral(n ast.Node) (lit *ast.ValueExpression, negative bool) {
	for {
		switch e := n.(type) {
		case *ast.ParenExpression:
			n = e.Expression
		case *ast.UnaryExpression:
			if e.Postfix {
				return nil, false
			}
			switch e.Operator.Type {
			case scanner.TokenTypeSUB:
				negative = !negative
				n = e.Expression
			case scanner.TokenTypeADD:
				n = e.Expression
			default:
				return nil, false
			}
		case *ast.ValueExpression:
			switch e.Token.Type {
			case scanner.TokenTypeNumber, scanner.TokenTypeFloat:
				return e, negative
			}
			return nil, false
		default:
			return nil, false
		}
	}
}

// literalFitsType reports whether node is a numeric literal whose value is
// representable in the target type, allowing e.g. `var a: uint8 = 200` or
// `var f: float64 = 1.5` without an explicit cast.
func literalFitsType(n ast.Node, target types.Type) bool {
	if target == nil {
		return false
	}
	d, ok := numericKinds[target.GetName()]
	if !ok {
		return false
	}
	lit, negative := unwrapNumericLiteral(n)
	if lit == nil {
		return false
	}

	if lit.Token.Type == scanner.TokenTypeFloat {
		// Float literals fit any float type (float64 exactly; float32 with
		// the usual rounding, same as a float32-typed literal today).
		return d.float
	}

	value, ok := lit.Token.Value.(int64)
	if !ok {
		return false
	}
	if negative {
		value = -value
	}

	if d.float {
		return true
	}
	if d.unsigned {
		if value < 0 {
			return false
		}
		if d.bits == 64 {
			return true
		}
		return value <= (int64(1)<<uint(d.bits))-1
	}
	min := int64(math.MinInt64)
	max := int64(math.MaxInt64)
	if d.bits < 64 {
		min = -(int64(1) << uint(d.bits-1))
		max = (int64(1) << uint(d.bits-1)) - 1
	}
	return value >= min && value <= max
}

// isAssignable reports whether a value produced by srcNode (with type src)
// can be used where dst is expected: exact type match, an in-range numeric
// literal, or a safe implicit numeric widening.
func isAssignable(srcNode ast.Node, src, dst types.Type) bool {
	if src == nil || dst == nil {
		return false
	}
	src = types.LazyResolve(src)
	dst = types.LazyResolve(dst)
	if dst.IsEqual(src) {
		return true
	}
	if literalFitsType(srcNode, dst) {
		return true
	}
	return isSafeNumericWidening(src, dst)
}

// checkBoolCondition reports an error when a condition expression has a
// known non-bool type. Unresolvable types are skipped: they are either
// already reported as undefined identifiers or (in for-loop init clauses)
// not yet in scope when the loop node itself is visited.
func (v *visitor) checkBoolCondition(expr ast.Expression) {
	if expr == nil {
		return
	}
	typ := types.LazyResolve(v.getTypeForNode(expr))
	if typ == nil {
		return
	}
	if _, unknown := typ.(types.UnknownType); unknown {
		return
	}
	if !typ.IsEqual(types.BoolType) {
		v.emitError(expr, fmt.Sprintf(
			"non-bool %s (type %s) used as condition",
			expr,
			typ.GetName(),
		), true)
	}
}

// checkConstAssignment reports an error when assigning to a variable that
// was declared const (its declaring initialization is allowed).
func (v *visitor) checkConstAssignment(n ast.Node, ident *ast.Identifier) {
	item := v.scope.Get(ident.Text, true)
	varDecl, ok := item.(*ast.VariableDeclaration)
	if !ok || !varDecl.Constant {
		return
	}
	if details := v.scope.GetDetails(ident.Text, true); details != nil && !details.Initialized {
		// First assignment initializes a const declared without a value.
		return
	}
	v.emitError(n, fmt.Sprintf("cannot assign to constant %s", ident.Text), true)
}

// numericOperandsCompatible reports whether two operands of a binary or
// comparison expression are compatible, and if so which type the combined
// operation has (the type both operands convert to).
func numericOperandsCompatible(aNode, bNode ast.Node, a, b types.Type) (bool, types.Type) {
	if a == nil || b == nil {
		return false, a
	}
	a = types.LazyResolve(a)
	b = types.LazyResolve(b)
	if a.IsEqual(b) {
		return true, a
	}
	// A literal adapts to the other operand's type.
	if literalFitsType(bNode, a) {
		return true, a
	}
	if literalFitsType(aNode, b) {
		return true, b
	}
	if isSafeNumericWidening(a, b) {
		return true, b
	}
	if isSafeNumericWidening(b, a) {
		return true, a
	}
	return false, a
}
