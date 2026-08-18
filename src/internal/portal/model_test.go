package portal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var apps = []App{
	{
		Name: "chat",
		Environments: []Environment{
			{Name: "local", URL: "http://127.0.0.1:1339"},
			{Name: "prod", URL: "https://chat.example.dev", Gated: true},
		},
	},
	{Name: "nowhere"}, // declares no environments
}

func TestSelectionStartsOnFirstEnvironment(t *testing.T) {
	selection := NewSelection(apps)
	env, ok := selection.Current(apps[0])
	if !ok || env.Name != "local" {
		t.Fatalf("want local selected, got %q ok=%v", env.Name, ok)
	}
}

func TestCycleWrapsBothWays(t *testing.T) {
	selection := NewSelection(apps)

	selection.Cycle(apps[0], 1)
	if env, _ := selection.Current(apps[0]); env.Name != "prod" {
		t.Fatalf("forward: want prod, got %q", env.Name)
	}
	selection.Cycle(apps[0], 1)
	if env, _ := selection.Current(apps[0]); env.Name != "local" {
		t.Fatalf("forward wrap: want local, got %q", env.Name)
	}
	selection.Cycle(apps[0], -1)
	if env, _ := selection.Current(apps[0]); env.Name != "prod" {
		t.Fatalf("backward wrap: want prod, got %q", env.Name)
	}
}

func TestCycleOnAppWithNoEnvironmentsIsSafe(t *testing.T) {
	selection := NewSelection(apps)
	selection.Cycle(apps[1], 1) // must not panic or divide by zero
	if _, ok := selection.Current(apps[1]); ok {
		t.Fatal("an app with no environments must not resolve one")
	}
}

func TestRowsSkipAppsWithNoEnvironments(t *testing.T) {
	rows := Rows(apps, NewSelection(apps), map[string]Status{})
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].EnvironmentCount != 2 || rows[0].EnvironmentIndex != 0 {
		t.Fatalf("want 1/2 affordance, got %d/%d",
			rows[0].EnvironmentIndex, rows[0].EnvironmentCount)
	}
}

func TestProbeReportsUpForSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	status := NewProber(time.Second).Probe(t.Context(), Environment{URL: server.URL})
	if status.State != StateUp {
		t.Fatalf("want up, got %s (%s)", status.State, status.Detail)
	}
}

// An SSO gate answers a redirect to its login origin. Following it lands on a
// 200 login page, which is exactly how a gated host gets misreported as up.
func TestProbeReportsGatedForAnSSORedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("location", "https://acme.cloudflareaccess.com/cdn-cgi/access/login/app")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	status := NewProber(time.Second).Probe(t.Context(), Environment{URL: server.URL})
	if status.State != StateGated {
		t.Fatalf("want gated, got %s (%s)", status.State, status.Detail)
	}
}

func TestProbeReportsDownForServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	status := NewProber(time.Second).Probe(t.Context(), Environment{URL: server.URL})
	if status.State != StateDown || status.Detail != "HTTP 500" {
		t.Fatalf("want down/HTTP 500, got %s/%s", status.State, status.Detail)
	}
}

func TestProbeNamesARefusedConnection(t *testing.T) {
	// Port 1 on loopback is reliably closed, so this exercises the dial path
	// rather than a status code.
	status := NewProber(time.Second).Probe(t.Context(), Environment{URL: "http://127.0.0.1:1"})
	if status.State != StateDown {
		t.Fatalf("want down, got %s", status.State)
	}
	if status.Detail != "not running" && status.Detail != "unreachable" {
		t.Fatalf("want an actionable detail, got %q", status.Detail)
	}
}

func TestProbePrefersTheHealthOverride(t *testing.T) {
	var probed string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probed = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	env := Environment{URL: server.URL + "/ui", Health: server.URL + "/health"}
	NewProber(time.Second).Probe(t.Context(), env)
	if probed != "/health" {
		t.Fatalf("want /health probed, got %q", probed)
	}
}

func TestProbeAllKeysEveryEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	local := []App{{Name: "chat", Environments: []Environment{
		{Name: "local", URL: server.URL},
		{Name: "prod", URL: server.URL},
	}}}
	statuses := NewProber(time.Second).ProbeAll(t.Context(), local)
	for _, key := range []string{"chat/local", "chat/prod"} {
		if statuses[key].State != StateUp {
			t.Fatalf("%s: want up, got %s", key, statuses[key].State)
		}
	}
}

func TestUnconfiguredURLIsUnknownNotDown(t *testing.T) {
	status := NewProber(time.Second).Probe(t.Context(), Environment{Name: "local"})
	if status.State != StateUnknown {
		t.Fatalf("want unknown, got %s", status.State)
	}
}

// A service that answers 401 is up and enforcing auth. Reporting it as down is
// the same false alarm as scoring an SSO login page as healthy.
func TestProbeTreatsUnauthorizedAsGated(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))
		status := NewProber(2*time.Second).Probe(context.Background(), Environment{URL: server.URL})
		if status.State != StateGated {
			t.Errorf("HTTP %d: got state %q, want %q", code, status.State, StateGated)
		}
		if status.Detail != "auth required" {
			t.Errorf("HTTP %d: got detail %q, want %q", code, status.Detail, "auth required")
		}
		server.Close()
	}
}

// A 404 stays down: the host is reachable but the route a reader was told to
// open does not exist, which is a real problem rather than an auth gate.
func TestProbeKeepsNotFoundDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	status := NewProber(2*time.Second).Probe(context.Background(), Environment{URL: server.URL})
	if status.State != StateDown {
		t.Fatalf("got state %q, want %q", status.State, StateDown)
	}
}

// The token goes only to environments declared gated. Localhost and public
// hosts in the same catalogue must not receive a credential.
func TestProbeSendsAccessHeadersOnlyWhenGated(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		gated bool
		want  string
	}{
		{name: "gated", gated: true, want: "id-123"},
		{name: "ungated", gated: false, want: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var seen string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("CF-Access-Client-Id")
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			prober := NewProber(2 * time.Second)
			prober.Access = map[string]string{"CF-Access-Client-Id": "id-123"}
			prober.Probe(context.Background(), Environment{URL: server.URL, Gated: testCase.gated})

			if seen != testCase.want {
				t.Errorf("header sent = %q, want %q", seen, testCase.want)
			}
		})
	}
}

// With a valid token the gate answers 200, so a gated environment reads up
// rather than gated — the whole point of giving the portal a token.
func TestProbeReportsUpWhenTheTokenIsAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("CF-Access-Client-Id") == "" {
			w.Header().Set("location", "https://example.cloudflareaccess.com/cdn-cgi/access/login/x")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prober := NewProber(2 * time.Second)
	prober.Access = map[string]string{"CF-Access-Client-Id": "id-123"}

	gated := prober.Probe(context.Background(), Environment{URL: server.URL, Gated: false})
	if gated.State != StateGated {
		t.Fatalf("without the token: got %q, want %q", gated.State, StateGated)
	}
	up := prober.Probe(context.Background(), Environment{URL: server.URL, Gated: true})
	if up.State != StateUp {
		t.Fatalf("with the token: got %q, want %q", up.State, StateUp)
	}
}
