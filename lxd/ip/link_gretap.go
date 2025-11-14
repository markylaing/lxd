package ip

import "context"

// Gretap represents arguments for link of type gretap.
type Gretap struct {
	Link
	Local  string
	Remote string
}

// additionalArgs generates gretap specific arguments.
func (g *Gretap) additionalArgs() []string {
	return []string{"local", g.Local, "remote", g.Remote}
}

// Add adds new virtual link.
func (g *Gretap) Add(ctx context.Context) error {
	return g.add(ctx, "gretap", g.additionalArgs())
}
