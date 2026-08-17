package portal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// State is the traffic light. Three values, not two: a host behind an SSO
// challenge is reachable but not usable, and reporting that as either "up" or
// "down" would be a lie the reader then has to debug.
type State string

const (
	StateUnknown State = "unknown" // not probed yet
	StateUp      State = "up"
	StateDown    State = "down"
	StateGated   State = "gated" // reachable, but an auth gate answered
)

// Status is one probe result.
type Status struct {
	State   State         `json:"state"`
	Detail  string        `json:"detail"`
	Latency time.Duration `json:"-"`
	// LatencyMS is the wire form; Latency is not JSON-friendly.
	LatencyMS int64 `json:"latencyMs"`
}

// Light is the single glyph a front end renders.
func (s Status) Light() string {
	switch s.State {
	case StateUp:
		return "●"
	case StateDown:
		return "●"
	case StateGated:
		return "◐"
	default:
		return "○"
	}
}

// Prober probes environments. It exists as a type so tests can substitute a
// transport without a live network.
type Prober struct {
	Client  *http.Client
	Timeout time.Duration
}

// NewProber builds a prober that does not follow redirects.
//
// Not following them is the point: an Access/SSO gate answers 302 to a login
// origin, and a client that follows it lands on a 200 login page. That is how
// a gated host gets misreported as healthy.
func NewProber(timeout time.Duration) *Prober {
	return &Prober{
		Timeout: timeout,
		Client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Probe resolves one environment's status.
func (p *Prober) Probe(ctx context.Context, env Environment) Status {
	target := env.Probe()
	if target == "" {
		return Status{State: StateUnknown, Detail: "no url configured"}
	}

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	started := time.Now()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Status{State: StateDown, Detail: "bad url"}
	}
	request.Header.Set("accept", "*/*")

	response, err := p.Client.Do(request)
	elapsed := time.Since(started)
	if err != nil {
		return withLatency(Status{State: StateDown, Detail: describeDialError(err)}, elapsed)
	}
	defer func() { _ = response.Body.Close() }()

	location := response.Header.Get("location")
	if isAuthChallenge(response.StatusCode, location) {
		return withLatency(Status{State: StateGated, Detail: "sign-in required"}, elapsed)
	}
	// 401/403 is the API-shaped equivalent of an SSO redirect: the service is
	// answering, it just will not serve an anonymous probe. Scoring that "down"
	// sends someone to debug a healthy worker.
	if response.StatusCode == http.StatusUnauthorized ||
		response.StatusCode == http.StatusForbidden {
		return withLatency(Status{State: StateGated, Detail: "auth required"}, elapsed)
	}
	if response.StatusCode >= 200 && response.StatusCode < 400 {
		return withLatency(Status{State: StateUp, Detail: fmt.Sprintf("%d", response.StatusCode)}, elapsed)
	}
	return withLatency(
		Status{State: StateDown, Detail: fmt.Sprintf("HTTP %d", response.StatusCode)},
		elapsed,
	)
}

// ProbeAll probes every environment of every app concurrently and returns
// results keyed by "app/environment". One slow host must not serialize the
// whole page.
func (p *Prober) ProbeAll(ctx context.Context, apps []App) map[string]Status {
	results := make(map[string]Status)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, app := range apps {
		for _, env := range app.Environments {
			wg.Add(1)
			go func(appName string, env Environment) {
				defer wg.Done()
				status := p.Probe(ctx, env)
				mu.Lock()
				results[Key(appName, env.Name)] = status
				mu.Unlock()
			}(app.Name, env)
		}
	}

	wg.Wait()
	return results
}

// Key identifies one app/environment pair in a status map.
func Key(app, environment string) string {
	return app + "/" + environment
}

func withLatency(status Status, elapsed time.Duration) Status {
	status.Latency = elapsed
	status.LatencyMS = elapsed.Milliseconds()
	return status
}

// isAuthChallenge reports a redirect to a recognized identity provider. Kept
// to well-known SSO hosts rather than "any cross-origin redirect", so an
// ordinary marketing redirect is not mislabeled as a gate.
func isAuthChallenge(statusCode int, location string) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound,
		http.StatusSeeOther, http.StatusTemporaryRedirect:
	default:
		return false
	}
	for _, host := range []string{
		"cloudflareaccess.com",
		"accounts.google.com",
		"login.microsoftonline.com",
		"okta.com",
		"auth0.com",
	} {
		if strings.Contains(location, host) {
			return true
		}
	}
	return false
}

// describeDialError turns Go's transport errors into something a reader can
// act on. "connection refused" means start the server; a DNS failure means
// check the hostname; a timeout means it is reachable but not answering.
func describeDialError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timed out"
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "connection refused"):
		return "not running"
	case strings.Contains(message, "no such host"):
		return "unknown host"
	case strings.Contains(message, "certificate"):
		return "tls error"
	case strings.Contains(message, "context deadline exceeded"):
		return "timed out"
	default:
		return "unreachable"
	}
}
