package mocks

import "context"

// Pinger is a test double for handler.Pinger. Ping returns Err.
type Pinger struct {
	Err error
}

func (m *Pinger) Ping(context.Context) error {
	return m.Err
}
