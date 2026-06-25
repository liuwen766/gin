# Gin Web Framework - Project Context

## Overview
- **Module**: `github.com/gin-gonic/gin` v1.12.0
- **Language**: Go 1.25+
- **Type**: High-performance HTTP web framework
- **License**: MIT

## Architecture

### Core Components

```
Engine (gin.go)
├── RouterGroup (routergroup.go)     - Route grouping, middleware chaining
├── Context (context.go)              - Per-request context (~1540 lines)
├── Tree (tree.go)                    - Radix tree URL router (~950 lines)
├── ResponseWriter (response_writer.go) - Wraps http.ResponseWriter
├── Recovery (recovery.go)            - Panic recovery middleware
├── Logger (logger.go)                - Request logging middleware
├── Auth (auth.go)                    - Basic/BasicProxy auth middleware
├── Errors (errors.go)                - Error type system (ErrorType, errorMsgs)
├── Path (path.go)                    - URL path cleaning utilities
├── FS (fs.go)                        - File system wrappers (OnlyFilesFS)
├── Utils (utils.go)                  - H map, Bind, WrapF/WrapH, helpers
├── Debug (debug.go)                  - Debug print utilities
├── Mode (mode.go)                    - debug/release/test mode
└── Deprecated (deprecated.go)        - Deprecated API surface

binding/            - Request data binding & validation
  binding.go        - Binding interface, Default() content-type dispatch
  form_mapping.go   - Form/query struct mapping engine (~550 lines)
  default_validator.go - go-playground/validator integration
  json.go, xml.go, yaml.go, toml.go, form.go, query.go, header.go, uri.go, etc.

render/             - Response rendering
  render.go         - Render interface
  json.go           - JSON/IndentedJSON/SecureJSON/JsonpJSON/AsciiJSON/PureJSON
  redirect.go       - Redirect renderer
  html.go, xml.go, yaml.go, toml.go, protobuf.go, bson.go, msgpack.go, data.go, reader.go, pdf.go

codec/json/         - JSON codec abstraction (compile-time selection)
  api.go            - Common API interface
  sonic.go          - bytedance/sonic (default)
  go_json.go        - goccy/go-json
  jsoniter.go       - json-iterator/go
  json.go           - stdlib encoding/json (fallback)

internal/
  bytesconv/        - Zero-allocation string/bytes conversion (unsafe)
  fs/               - FileSystem adapter

ginS/               - Singleton mode (gins.go)
```

### Request Flow

1. `Engine.ServeHTTP(w, req)` - http.Handler entry point
2. Lazy `routeTreesUpdated.Do(updateRouteTrees)` on first request
3. Context acquired from `sync.Pool`, reset, populated
4. `engine.handleHTTPRequest(c)`:
   - Resolve path (UseEscapedPath / UseRawPath / RemoveExtraSlash)
   - Linear scan `engine.trees` by HTTP method
   - Radix tree `root.getValue()` for route matching + param extraction
   - On match: set handlers, `c.Next()`, `WriteHeaderNow()`
   - On miss: RedirectTrailingSlash → RedirectFixedPath → HandleMethodNotAllowed → 404
5. Context returned to pool

### Key Data Structures

- **`Engine`**: Embeds `RouterGroup`; holds `methodTrees` (slice of `methodTree`), `sync.Pool` for contexts, config flags
- **`Context`**: Mutable per-request state — Request, Writer, Params, handlers chain, index (int8), Keys (map), Errors, query/form cache
- **`node`** (tree.go): Radix tree node — path, indices, wildChild, nType (static/root/param/catchAll), children, handlers, fullPath
- **`responseWriter`**: Wraps `http.ResponseWriter`; tracks status, size, written state; implements Hijacker/Flusher/CloseNotifier/Pusher
- **`HandlersChain`**: `[]HandlerFunc`; `Last()` returns the terminal handler
- **`Error`**: Wraps `error` with `ErrorType` bitmask (Bind, Render, Private, Public) and Meta

### Router (Radix Tree)

- Based on julienschmidt/httprouter
- Supports `:param` and `*catchAll` wildcards
- `skippedNodes` mechanism for backtracking during route matching
- `RedirectTrailingSlash` (default true): 301/307 redirect for trailing slash mismatch
- `RedirectFixedPath`: case-insensitive path correction
- `HandleMethodNotAllowed`: 405 with Allow header
- `RemoveExtraSlash`: normalize multiple slashes

### Binding System

- `Binding` interface: `Name() + Bind(*http.Request, any)`
- `BindingBody`: adds `BindBody([]byte, any)`
- `BindingUri`: adds `BindUri(map[string][]string, any)`
- `Default(method, contentType)` dispatches: GET→Form, JSON→JSON, XML→XML, etc.
- `form_mapping.go`: Reflection-based struct mapping with tag support (`form:`, `uri:`, `header:`)
- Custom types via `BindUnmarshaler` interface
- `collection_format` tag: csv, ssv, tsv, pipes, multi
- `time_format` tag: RFC3339, unix, unixmilli, etc.
- `default` tag for default values
- Validator: `go-playground/validator/v10` with `binding` tag name

### JSON Codec Selection

- Compile-time: `sonic` (default) → `go_json` → `jsoniter` → `stdlib`
- Controlled by build tags in `codec/json/api.go`

### Notable Patterns & Edge Cases

- **Context pooling**: `sync.Pool` reuses contexts; `reset()` clears all fields
- **Context.Copy()**: Creates a goroutine-safe copy; sets `index = abortIndex`
- **Context.Request.Context() integration**: `ContextWithFallback` engine flag; `Value()` first checks `c.Keys`, then falls back to `c.Request.Context()`
- **ClientIP()**: Trusted platform → AppEngine → RemoteAddr → trusted proxy check → Forwarded headers (X-Forwarded-For, X-Real-IP)
- **Redirect()**: Calls `c.Render(-1, render.Redirect{...})` — passes -1 status code to Render, render.Redirect then validates 3xx range
- **Negotiate()**: Content negotiation → defaults to first offered format if Accept header empty
- **ErrorLogger()**: Logs errors via `c.JSON(-1, errors)` — also passes -1 status code
- **Logger**: `SkipPaths` for path exclusion, `Skip` func for custom skip logic, `SkipQueryString`
- **Recovery**: Handles broken pipe (EPIPE, ECONNRESET, ErrAbortHandler) specially
- **Static file serving**: `Static()` uses `OnlyFilesFS` to prevent directory listing
- **H2C & HTTP/3**: `UseH2C` flag; `RunQUIC()` method for QUIC protocol
- **`abortIndex`**: `math.MaxInt8 >> 1` (63); index ≥ abortIndex means aborted
- **`safeInt8`/`safeUint16`**: Guard against overflow when converting handler counts

### Test Coverage

- Main test files: `gin_test.go`, `context_test.go` (~122KB), `routes_test.go`, `tree_test.go`, `logger_test.go`, `recovery_test.go`, `routergroup_test.go`, `utils_test.go`, `auth_test.go`, `errors_test.go`, `response_writer_test.go`, `debug_test.go`, `mode_test.go`, `fs_test.go`, `path_test.go`, `deprecated_test.go`
- Integration: `gin_integration_test.go`, `githubapi_test.go`
- Binding tests: `binding_test.go`, `form_mapping_test.go`, `default_validator_test.go`, etc.
- Test framework: `stretchr/testify`