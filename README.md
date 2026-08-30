# gerrit-sdk-go

A **generated Go SDK** for the Gerrit Code Review REST API (the `gerritclient`
package), produced from Gerrit's statically generated **OpenAPI 3.1** document. No
hand-written request/response types: every operation and model comes from the spec, so
the client never drifts from the server.

## The pipeline (end to end)

```
  gerrit                     gerrit-sdk-go                  examples/
  (emit the spec)      -->    (this repo: the SDK)    -->    (consume the SDK)
  parse-only OpenAPI          openapi-generator (go)         go run, live calls,
  emitter                     + XSSI transport               colored output
```

1. **gerrit emits the spec.** A parse-only emitter (`java/com/google/gerrit/openapi/**`)
   reads the server's REST bindings via the javac Compiler Tree API — no running
   server, no reflection — and writes an OpenAPI 3.1 JSON.
2. **This repo pins that spec.** `rest-api-openapi.json` is a checked-in snapshot of
   that target's output. Its `info.version` / `info.license` / `servers` come straight
   from the emitter.
3. **`generate.sh` generates the package.** openapi-generator (go) turns the spec into
   `gerritclient/`.
4. **A consumer reuses it by module path.** `go get github.com/davido/gerrit-sdk-go/v3`
   (or run the bundled example) — see [Use it](#use-it).

The whole story demonstrates feasibility for Gerrit issue
[40011133](https://issues.gerritcodereview.com/issues/40011133) ("Consider using
Swagger from OpenApi for REST API").

## Version

Generated from **Gerrit 3.15.0-SNAPSHOT** and tagged **`v3.15.0-SNAPSHOT`** — the tag
mirrors the Gerrit version, so consumers pin the exact server generation they target.

## What's in this repo

- `gerritclient/` — the generated package: **341 operations** across **7 API services**
  and **278 model types**, over `net/http`.
- `gerritxssi/` — a tiny hand-written `http.RoundTripper` that strips Gerrit's `)]}'`
  XSSI guard (the one Gerrit-specific step; see below). Lives outside the generated
  code, so regeneration never touches it.
- `examples/get-change-detail/` — a runnable example: an anonymous `GET /changes/{id}`
  rendered as a colored, Web-UI-style summary using Gerrit's own palette.
- `rest-api-openapi.json`, `generate.sh` — the pinned spec and the generation script.

## Regenerate

```bash
./generate.sh [path-or-url]      # default: ./rest-api-openapi.json
```

The module is never hand-maintained: to track a new Gerrit version, refetch the
spec (pass a URL to `generate.sh`) and regenerate.

## The Gerrit-specific handling

The spec is consumed as-is; only three things are needed around the generator, and
**none of them patch generated code**:

1. **XSSI guard** — every Gerrit JSON body starts with `)]}'` on its own line, which is
   not valid JSON and not expressible in OpenAPI. Stripped by the `gerritxssi`
   transport (`gerritxssi.Client()`), the only irreducible Gerrit-specific step.
2. **`enumClassPrefix=true`** (generator flag) — prefixes enum constants with their type
   name; without it the go generator emits bare names (`OK`, `NONE`, `ALL`, …) that
   collide at package scope.
3. **`--parameter-name-mappings r=regexFilter`** (generator flag) — the plugin-list
   regex query param is named `r` (Gerrit's `@Option(name="-r")`), but the go generator
   uses `r` as its request-builder receiver, so `func (r …) R(r string)` would shadow
   it. The mapping renames the Go identifier; the wire query string is still `?r=`.

The case-colliding `O`/`o` query params are handled by the go generator on its own
(distinct struct fields), so — unlike the Rust SDK — no query patch is needed.

## Build & test

```bash
go build ./...        # build the SDK, transport, and examples
go vet ./...          # static checks
go test ./...         # unit tests (none yet; the command is green)
```

### Test locally — no publish needed

The example lives *inside* this module, so its `github.com/davido/gerrit-sdk-go/v3/…`
imports resolve to the **local source**; Go never hits the network. Run it against a
live Gerrit right now:

```bash
go run ./examples/get-change-detail -- --change 622261
```

No `replace` directive is required — being in the same module, local resolution is
automatic (unlike a separate consumer module, which would need one to test against
unpublished code).

### Test from GitHub — after publishing

Once the repo is pushed and tagged, the **same import path** is fetched from the VCS for
any consumer, anywhere:

```bash
go run github.com/davido/gerrit-sdk-go/v3/examples/get-change-detail@v3.15.0-SNAPSHOT -- --change 621763
```

**Rationale:** Go resolves imports by module path. Inside this module the path maps to
local files (instant, offline, no publish); outside it — or with an explicit `@version`
— Go downloads the tagged source from GitHub. So local development needs no publish;
external reuse is what needs the tag.

## Use it

```bash
# in a clone (local source):
go run ./examples/get-change-detail -- --change 622261

# straight from GitHub, no clone (after publish):
go run github.com/davido/gerrit-sdk-go/v3/examples/get-change-detail@latest -- --change 621763
```

In your own module:

```go
import (
	gc "github.com/davido/gerrit-sdk-go/v3/gerritclient"
	"github.com/davido/gerrit-sdk-go/v3/gerritxssi"
)

cfg := gc.NewConfiguration()
cfg.Servers = gc.ServerConfigurations{{URL: "https://gerrit-review.googlesource.com"}}
cfg.HTTPClient = gerritxssi.Client() // strip the )]}' XSSI guard
client := gc.NewAPIClient(cfg)

ci, _, err := client.ChangesAPI.
	GetChangesChangeId(context.Background(), "621763").
	O2([]string{"LABELS", "CURRENT_REVISION"}).
	Execute()
```

```bash
go get github.com/davido/gerrit-sdk-go/v3@v3.15.0-SNAPSHOT
```

Go modules publish by **git tag** — there is no artifact to upload; `go get` fetches the
tagged source directly from GitHub.

## Status

Prototype demonstrating feasibility for Gerrit issue
[40011133](https://issues.gerritcodereview.com/issues/40011133).

## License

Apache 2.0. See [LICENSE.txt](LICENSE.txt).
