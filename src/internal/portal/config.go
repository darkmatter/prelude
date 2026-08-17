// Package portal renders a launcher for a project's apps: one row per app,
// an environment selector, a health traffic light, and a clickable URL.
//
// Both front ends — the terminal UI and the local web server — are thin views
// over this package. Probing, status semantics, and environment selection live
// here so the two cannot disagree about whether something is up.
package portal

import (
	"time"

	"prelude/pkg/shared"
)

// DefaultTimeout bounds one health probe. A devshell launcher must stay
// responsive on a laptop that is off the VPN, so this is deliberately short.
const DefaultTimeout = 3 * time.Second

// Config is the Nix-generated payload. Field names mirror the module options
// in `src/prelude/portal.nix`; `shared.LoadJSON` rejects unknown fields, so the
// two cannot drift silently.
type Config struct {
	Project      string         `json:"project"`
	ColorProfile string         `json:"colorProfile"`
	Palette      shared.Palette `json:"palette"`
	MaxWidth     int            `json:"maxWidth"`
	// Listen is the default address for the web front end, e.g. "127.0.0.1:7777".
	Listen string `json:"listen"`
	// TimeoutMS bounds each probe; 0 uses DefaultTimeout.
	TimeoutMS int   `json:"timeoutMs"`
	Apps      []App `json:"apps"`
}

// App is one launchable surface with one row per environment.
type App struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// Environments are ordered as declared; the first is selected initially,
	// which is why local should be declared first in a project's config.
	Environments []Environment `json:"environments"`
}

// Environment is one deployment of an app.
type Environment struct {
	Name string `json:"name"`
	// URL is what gets opened and, unless Health is set, what gets probed.
	URL string `json:"url"`
	// Health overrides the probe target. A UI route often answers 200 for an
	// unauthenticated visitor while the thing you care about is a health
	// endpoint behind it.
	Health string `json:"health"`
	// Gated marks an environment fronted by an SSO/Access challenge. Such a
	// host answers a redirect to a login origin, which is neither up nor down
	// in any useful sense — see State for how that is reported.
	Gated bool `json:"gated"`
}

// Timeout resolves the configured probe budget.
func (c Config) Timeout() time.Duration {
	if c.TimeoutMS <= 0 {
		return DefaultTimeout
	}
	return time.Duration(c.TimeoutMS) * time.Millisecond
}

// Probe target for one environment: Health when set, else URL.
func (e Environment) Probe() string {
	if e.Health != "" {
		return e.Health
	}
	return e.URL
}
