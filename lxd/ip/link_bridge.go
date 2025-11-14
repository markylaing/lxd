package ip

import "context"

// Bridge represents arguments for link device of type bridge.
type Bridge struct {
	Link
}

// Add adds new virtual link.
func (b *Bridge) Add(ctx context.Context) error {
	return b.add(ctx, "bridge", nil)
}
