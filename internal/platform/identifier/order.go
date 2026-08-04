package identifier

import "github.com/yywencs/courseforge/pkg/idgen"

// OrderIDGenerator adapts the existing UUIDv7 generator to
// constructor-injected IDGenerator ports.
type OrderIDGenerator struct{}

func NewOrderIDGenerator() OrderIDGenerator {
	return OrderIDGenerator{}
}

func (OrderIDGenerator) NewID() (string, error) {
	return idgen.NewOrderID()
}
