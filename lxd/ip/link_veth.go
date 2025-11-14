package ip

import "context"

// Veth represents arguments for link of type veth.
type Veth struct {
	Link
	Peer Link
}

// Add adds new virtual link.
func (veth *Veth) Add(ctx context.Context) error {
	return veth.add(ctx, "veth", append([]string{"peer"}, veth.Peer.args()...))
}
