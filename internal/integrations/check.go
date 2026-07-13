// Package integrations holds shared primitives for the external directory and
// workplace integrations (LDAP, EntraID, Robin, Geoapify).
package integrations

// Check is one row of a structured integration test: a named step with an
// "ok" | "warn" | "fail" status and a human-readable detail line. Rendered by
// the admin test modals.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// Result wraps a check list into the {ok, checks} payload the admin test
// modals expect. ok is true when no check failed (warnings are tolerated).
func Result(checks []Check) map[string]interface{} {
	ok := true
	for _, c := range checks {
		if c.Status == "fail" {
			ok = false
			break
		}
	}
	return map[string]interface{}{"ok": ok, "checks": checks}
}
