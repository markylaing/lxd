package ip

import "context"

// Dummy represents arguments for link device of type dummy.
type Dummy struct {
	Link
}

// Add adds new virtual link.
func (d *Dummy) Add(ctx context.Context) error {
	return d.add(ctx, "dummy", nil)
}
