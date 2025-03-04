package shadowapi

import "github.com/canonical/lxd/lxd/auth"

//go:generate go run ./generate/main.go ../../shared/api .
//go:generate goimports -w .

func (e *WithEntitlements) ReportEntitlements(entitlements []string) {
	properEntitlements := make([]auth.Entitlement, 0, len(entitlements))
	for _, e := range entitlements {
		properEntitlements = append(properEntitlements, auth.Entitlement(e))
	}

	e.AccessEntitlements = properEntitlements
}
