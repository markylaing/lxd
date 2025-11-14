package ip

import "context"

// Macvlan represents arguments for link of type macvlan.
type Macvlan struct {
	Link
	Mode string
}

// Add adds new virtual link.
func (macvlan *Macvlan) Add(ctx context.Context) error {
	return macvlan.add(ctx, "macvlan", []string{"mode", macvlan.Mode})
}
