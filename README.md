# brightencode-appkit

The shared scaffolding behind every Securexe/Brightencode applet: the
local HTTP+SSE server, job progress/cancellation, the Chrome "app mode"
window, and output-path helpers. Every applet in the catalog (gif-maker,
archive-manager, pdf-toolkit, ...) converges on the same shape — this
package is that shape, factored out once instead of copy-pasted and
silently drifting apart in ~20 repos.

An applet using this package only writes two things: the domain logic that
does the actual work (an `engine` package, usually wrapping an existing
CLI tool like ffmpeg or yt-dlp), and the one HTTP handler that kicks a job
off. Everything else — server plumbing, progress streaming, window
opening, hosted-mode support, idle self-shutdown — comes from here.

## Packages

- **`browser`** — `OpenIfNotHosted(appName, url)` opens a borderless
  Chrome "app mode" window pointed at the applet's local server, unless
  the launcher has set `SECUREXE_HOSTED` (meaning it's hosting the UI in
  its own native window instead). Also `Open`/`Reveal` for opening a
  finished file or revealing it in Finder/Explorer.
- **`jobs`** — `Registry`/`Job`/`Event`: an in-memory job registry an
  applet's handlers publish progress into, and the HTTP layer streams out
  over SSE.
- **`paths`** — `ResolveDownloadsDir` (default output location) and
  `UniquePath` (never overwrite an existing output file).
- **`server`** — `Server`, embedded into an applet's own server type. Its
  `Start` binds loopback, mounts the shared job-events/cancel/reveal/open
  routes plus your static frontend, and calls back into your code to
  mount whatever route actually starts a job.

## Usage: wiring a new applet

```go
// internal/server/server.go
package server

import (
    "context"
    "net/http"

    appkit "github.com/DavidMarsanic/brightencode-appkit/server"
    "github.com/DavidMarsanic/yourapp/engine"
    "github.com/DavidMarsanic/yourapp/web"
)

type Server struct {
    *appkit.Server
    Engine           *engine.Engine
    DefaultOutputDir string
}

func New(ctx context.Context, eng *engine.Engine, defaultOutputDir string) *Server {
    return &Server{
        Server:           appkit.New(ctx, 0), // 0 = appkit.DefaultIdleTimeout
        Engine:           eng,
        DefaultOutputDir: defaultOutputDir,
    }
}

func (s *Server) Start(port int) (string, error) {
    return s.Server.Start(port, web.Static, func(mux *http.ServeMux) {
        mux.HandleFunc("POST /api/jobs", s.handleCreateJob)
    })
}
```

```go
// internal/server/handlers.go — the one handler you actually write
func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
    // parse the request, kick off s.Engine.Whatever in a goroutine,
    // job, ctx := s.Jobs.Create(s.Ctx); job.Publish(jobs.Event{...})
    // as it progresses, appkit.WriteJSON(w, http.StatusOK, map[string]string{"jobId": job.ID})
}
```

```go
// cmd/yourapp/main.go
addr, err := srv.Start(*port)
...
if err := appkitserver.OpenWindow("yourapp", addr); err != nil { ... }
```

## Migrating an existing applet

1. `go get github.com/DavidMarsanic/brightencode-appkit@main` in the
   applet's `go.mod`.
2. Delete `internal/browser`, `internal/jobs`, `internal/paths`, and the
   boilerplate parts of `internal/server` (`server.go` entirely;
   `handlers.go` keeps only the app-specific handler(s) — compare against
   this package's `server/handlers.go` to see what's now shared).
3. Wrap your app's `Server` type around `*appkit/server.Server` as shown
   above; move any app-specific `jobs.Event` fields into
   `brightencode-appkit/jobs`' `Event` first if they're not already there
   (it's a union of every field every applet has needed so far).
4. Replace `browser.OpenAppWindow(addr)` calls in `main.go` with
   `browser.OpenIfNotHosted(appName, addr)` (now takes the app name — it
   used to be baked into the package).
5. `go build ./...` and `go vet ./...`; smoke-test the app still opens and
   completes a job end to end.

## New applet from an existing open-source CLI tool

Because everything above is fixed, wrapping a new CLI tool as a catalog
app reduces to one small, well-bounded piece: an `engine` package that
shells out to the tool and reports progress as `jobs.Event`s, plus the one
`handleCreateJob`-shaped handler that calls it. That's narrow enough to
hand to an agent directly: "wrap `<tool>` the way gif-maker wraps ffmpeg"
is close to a complete spec once there's one real example to point at.

Recommended next step once a couple of migrations have landed: pick the
cleanest migrated applet as the reference example and write a short
`NEW_APPLET.md` here distilling it into a checklist (go.mod, cmd/main.go,
engine package shape, web/static minimum, packaging/macos, brightencode.json)
so spinning up a wrapper for a new tool is a mechanical, mostly-automatable
task rather than a from-scratch design exercise each time.

## Versioning

No tagged releases — applets depend on this via
`go get github.com/DavidMarsanic/brightencode-appkit@main` (a pseudo-
version pin in `go.mod`), the same convention already used for
cross-applet engine imports (e.g. `clip-and-gif` importing `gif-maker`).
Re-run `go get ...@main` to pick up fixes.

## License

MIT — see [LICENSE](LICENSE).
