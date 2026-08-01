package identifier

import "prizeforge/pkg/idgen"

// OrderIDGenerator adapts the existing Snowflake-based generator to
// constructor-injected IDGenerator ports.
type OrderIDGenerator struct{}

func NewOrderIDGenerator() OrderIDGenerator {
	return OrderIDGenerator{}
}

func (OrderIDGenerator) NewID() (string, error) {
	return idgen.NewOrderID()
}
