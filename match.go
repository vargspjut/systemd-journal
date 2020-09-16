package journal

// Match describes a set of matches to be applied
// to a journal instance
type Match struct {
	expr []matchExpr
}

type matchOp int

const (
	matchOpField matchOp = iota
	matchOpAnd
	matchOpOr
)

type matchExpr struct {
	op     matchOp
	field  string
	values []string
}

// Match adds a match to the expression. Multiple values may be specified for each field
// in an OR-like fashion
func (m *Match) Match(field string, value ...string) *Match {

	expr := matchExpr{
		op:     matchOpField,
		field:  field,
		values: value,
	}

	m.expr = append(m.expr, expr)
	return m
}

// And creates a logical AND expression of the match that follows
func (m *Match) And() *Match {
	m.expr = append(m.expr, matchExpr{op: matchOpAnd})
	return m
}

// Or creates a logical OR expression of the match that follows
func (m *Match) Or() *Match {
	m.expr = append(m.expr, matchExpr{op: matchOpOr})
	return m
}

// NewMatch creates an empty Match expression
func NewMatch() *Match {
	return &Match{}
}
