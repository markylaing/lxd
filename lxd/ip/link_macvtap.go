package ip

import "context"

// Macvtap represents arguments for link of type macvtap.
type Macvtap struct {
	Macvlan
}

// Add adds new virtual link.
func (macvtap *Macvtap) Add(ctx context.Context) error {
	return macvtap.add(ctx, "macvtap", []string{"mode", macvtap.Mode})
}
