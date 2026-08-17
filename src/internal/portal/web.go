package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"time"
)

// Serve runs the web front end until ctx is cancelled.
//
// Every environment of every app is shown at once — the terminal front end
// selects one environment per row because a list has one line per app, but a
// page has room for the whole grid, and seeing local next to prod at a glance
// is the point of opening a browser instead.
func Serve(ctx context.Context, cfg Config, addr string) error {
	if addr == "" {
		addr = cfg.Listen
	}
	if addr == "" {
		addr = "127.0.0.1:7777"
	}

	prober := NewProber(cfg.Timeout())
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		statuses := prober.ProbeAll(r.Context(), cfg.Apps)
		w.Header().Set("content-type", "application/json")
		w.Header().Set("cache-control", "no-store")
		_ = json.NewEncoder(w).Encode(statuses)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// The first paint carries statuses so the page is useful before any
		// script runs; the poll below only keeps it fresh.
		statuses := prober.ProbeAll(r.Context(), cfg.Apps)
		w.Header().Set("content-type", "text/html; charset=utf-8")
		w.Header().Set("cache-control", "no-store")
		_, _ = w.Write([]byte(page(cfg, statuses)))
	})

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("portal: cannot listen on %s: %w", addr, err)
	}

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	fmt.Printf("portal: http://%s\n", listener.Addr())

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// stateColor maps a state onto a palette token. Kept beside the page template
// so the web and terminal front ends agree on what green means.
func stateColor(cfg Config, state State) string {
	switch state {
	case StateUp:
		return cfg.Palette.Success.String()
	case StateDown:
		return cfg.Palette.Error.String()
	case StateGated:
		return cfg.Palette.Warning.String()
	default:
		return cfg.Palette.Dim.String()
	}
}

func page(cfg Config, statuses map[string]Status) string {
	var body strings.Builder

	for _, app := range cfg.Apps {
		if len(app.Environments) == 0 {
			continue
		}
		body.WriteString(`<section class="app"><h2>` + html.EscapeString(app.Name) + `</h2>`)
		if app.Description != "" {
			body.WriteString(`<p class="desc">` + html.EscapeString(app.Description) + `</p>`)
		}
		body.WriteString(`<ul class="envs">`)
		for _, env := range app.Environments {
			key := Key(app.Name, env.Name)
			status := statuses[key]
			body.WriteString(fmt.Sprintf(
				`<li><span class="light" data-key="%s" style="color:%s">%s</span>`+
					`<a href="%s" target="_blank" rel="noreferrer">%s</a>`+
					`<span class="detail" data-detail="%s">%s</span></li>`,
				html.EscapeString(key),
				stateColor(cfg, status.State),
				status.Light(),
				html.EscapeString(env.URL),
				html.EscapeString(env.Name),
				html.EscapeString(key),
				html.EscapeString(detailText(status)),
			))
		}
		body.WriteString(`</ul></section>`)
	}

	if len(cfg.Apps) == 0 {
		body.WriteString(`<p class="desc">No apps configured — set <code>prelude.portal.apps</code>.</p>`)
	}

	pal := cfg.Palette
	return `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>` + html.EscapeString(cfg.Project) + ` — portal</title>
<style>
  :root {
    --bg: ` + pal.Bg.String() + `; --surface: ` + pal.Surface.String() + `;
    --fg: ` + pal.Fg.String() + `; --muted: ` + pal.Muted.String() + `;
    --dim: ` + pal.Dim.String() + `; --accent: ` + pal.Accent.String() + `;
    --border: ` + pal.Border.String() + `;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 2.5rem 1.5rem; background: var(--bg); color: var(--fg);
    font: 14px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace;
  }
  main { max-width: 60rem; margin: 0 auto; }
  h1 { font-size: 1.1rem; letter-spacing: .08em; text-transform: uppercase; margin: 0; }
  .sub { color: var(--muted); margin: .25rem 0 2rem; }
  .grid { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fill, minmax(17rem, 1fr)); }
  .app {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 10px; padding: 1rem 1.1rem;
  }
  h2 { font-size: .95rem; margin: 0; }
  .desc { color: var(--muted); margin: .3rem 0 .8rem; font-size: .82rem; }
  .envs { list-style: none; margin: 0; padding: 0; }
  .envs li { display: flex; align-items: baseline; gap: .55rem; padding: .22rem 0; }
  .light { font-size: 1rem; line-height: 1; }
  a { color: var(--accent); text-decoration: none; border-bottom: 1px solid transparent; }
  a:hover { border-bottom-color: var(--accent); }
  .detail { color: var(--dim); font-size: .75rem; margin-left: auto; }
  footer { color: var(--dim); margin-top: 2rem; font-size: .78rem; }
</style></head>
<body><main>
  <h1>` + html.EscapeString(cfg.Project) + ` portal</h1>
  <p class="sub">Click to launch. The light is a live health probe.</p>
  <div class="grid">` + body.String() + `</div>
  <footer>Refreshes every 5s · <span id="stamp">just now</span></footer>
</main>
<script>
  const colors = ` + colorMapJSON(cfg) + `;
  const lights = { up: "●", down: "●", gated: "◐", unknown: "○" };
  async function refresh() {
    try {
      const res = await fetch("/api/status", { cache: "no-store" });
      if (!res.ok) return;
      const statuses = await res.json();
      for (const [key, status] of Object.entries(statuses)) {
        const light = document.querySelector('.light[data-key="' + CSS.escape(key) + '"]');
        if (light) {
          light.style.color = colors[status.state] || colors.unknown;
          light.textContent = lights[status.state] || lights.unknown;
        }
        const detail = document.querySelector('.detail[data-detail="' + CSS.escape(key) + '"]');
        if (detail) {
          detail.textContent = status.latencyMs
            ? status.detail + " · " + status.latencyMs + "ms"
            : status.detail;
        }
      }
      document.getElementById("stamp").textContent = new Date().toLocaleTimeString();
    } catch (_) {
      // A failed poll leaves the last known lights in place rather than
      // blanking the page; the stamp going stale is the tell.
    }
  }
  setInterval(refresh, 5000);
</script>
</body></html>`
}

func colorMapJSON(cfg Config) string {
	encoded, err := json.Marshal(map[string]string{
		string(StateUp):      stateColor(cfg, StateUp),
		string(StateDown):    stateColor(cfg, StateDown),
		string(StateGated):   stateColor(cfg, StateGated),
		string(StateUnknown): stateColor(cfg, StateUnknown),
	})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func detailText(status Status) string {
	if status.LatencyMS > 0 {
		return fmt.Sprintf("%s · %dms", status.Detail, status.LatencyMS)
	}
	return status.Detail
}
