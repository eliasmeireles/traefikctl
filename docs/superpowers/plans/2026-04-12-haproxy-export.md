# HAProxy Export (Import) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `traefikctl haproxy export` command that reads an HAProxy config (from a file path or base64-encoded string), converts frontends/backends/listen blocks to Traefik dynamic YAML configs, skipping blocks whose ports are already registered (with a warning).

**Architecture:** Parse the HAProxy config into typed structs (Frontend, Backend, Listen), ignoring `global` and `defaults` sections. Convert HTTP frontend+backend pairs to Traefik HTTP routers/services and TCP listen blocks to Traefik TCP routers/services. Write one YAML file per frontend/listen block into `/etc/traefik/dynamic/`. Port conflicts are detected within the imported file and against existing dynamic configs; conflicting blocks are warned and skipped.

**Tech Stack:** Go, `github.com/spf13/cobra`, `gopkg.in/yaml.v3`, `github.com/stretchr/testify`, standard library `encoding/base64`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/cmd/haproxy.go` | Parent `haproxy` cobra command registered on root |
| Create | `internal/cmd/haproxy_parser.go` | Parse raw HAProxy config text into typed structs |
| Create | `internal/cmd/haproxy_export.go` | `export` subcommand: input handling, conflict check, conversion, file writing |
| Create | `internal/cmd/haproxy_parser_test.go` | Unit tests for the parser |
| Create | `internal/cmd/haproxy_export_test.go` | Unit tests for export logic |

---

### Task 1: HAProxy config data structures

**Files:**
- Create: `internal/cmd/haproxy_parser.go`

- [ ] **Step 1: Write the failing test for data structure existence**

```go
// internal/cmd/haproxy_parser_test.go
package cmd

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestHAProxyDataStructures(t *testing.T) {
    t.Run("must create Frontend with expected fields", func(t *testing.T) {
        f := HAProxyFrontend{
            Name:           "hapctl-traefik-http",
            Binds:          []string{"*:80"},
            Mode:           "http",
            ACLs:           []HAProxyACL{{Name: "host_fs", Condition: `hdr(host) -i fs.example.com`}},
            UseBackends:    []HAProxyUseBackend{{Backend: "be-fs", ACLName: "host_fs"}},
            DefaultBackend: "be-default",
        }
        require.Equal(t, "hapctl-traefik-http", f.Name)
        require.Equal(t, "http", f.Mode)
        require.Len(t, f.ACLs, 1)
    })

    t.Run("must create Backend with expected fields", func(t *testing.T) {
        b := HAProxyBackend{
            Name:    "be-default",
            Mode:    "http",
            Balance: "roundrobin",
            Servers: []HAProxyServer{{Name: "s1", Address: "127.0.0.1:8080", Options: "check"}},
        }
        require.Equal(t, "be-default", b.Name)
        require.Len(t, b.Servers, 1)
        require.Equal(t, "127.0.0.1:8080", b.Servers[0].Address)
    })

    t.Run("must create Listen with expected fields", func(t *testing.T) {
        l := HAProxyListen{
            Name:    "hapctl-game",
            Binds:   []string{"*:7777"},
            Mode:    "tcp",
            Balance: "roundrobin",
            Servers: []HAProxyServer{{Name: "srv", Address: "127.0.0.1:30777", Options: "check"}},
        }
        require.Equal(t, "hapctl-game", l.Name)
        require.Equal(t, "tcp", l.Mode)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd /home/eliasmeireles/workspace/personal/projects/traefikctl
go test ./internal/cmd/... -run TestHAProxyDataStructures -v
```
Expected: FAIL — `HAProxyFrontend`, `HAProxyBackend`, `HAProxyListen` undefined.

- [ ] **Step 3: Create the data structures**

```go
// internal/cmd/haproxy_parser.go
package cmd

// HAProxyConfig holds all parsed sections from an HAProxy config file.
// The global and defaults sections are intentionally ignored.
type HAProxyConfig struct {
    Frontends []HAProxyFrontend
    Backends  []HAProxyBackend
    Listens   []HAProxyListen
}

// HAProxyFrontend represents an HAProxy frontend block (HTTP mode).
type HAProxyFrontend struct {
    Name           string
    Binds          []string
    Mode           string
    ACLs           []HAProxyACL
    UseBackends    []HAProxyUseBackend
    DefaultBackend string
}

// HAProxyACL represents a named ACL definition inside a frontend.
type HAProxyACL struct {
    Name      string // e.g. "host_fileserver"
    Condition string // e.g. "hdr(host) -i fileserver.solutionstk.com"
}

// HAProxyUseBackend represents a conditional backend selection rule.
type HAProxyUseBackend struct {
    Backend string // backend name
    ACLName string // referenced ACL name
}

// HAProxyBackend represents an HAProxy backend block.
type HAProxyBackend struct {
    Name    string
    Mode    string
    Balance string
    Servers []HAProxyServer
}

// HAProxyListen represents an HAProxy listen block (combined frontend+backend, typically TCP).
type HAProxyListen struct {
    Name    string
    Binds   []string
    Mode    string
    Balance string
    Servers []HAProxyServer
}

// HAProxyServer represents a server entry inside a backend or listen block.
type HAProxyServer struct {
    Name    string // logical name for the server
    Address string // host:port
    Options string // e.g. "check"
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run TestHAProxyDataStructures -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_parser.go internal/cmd/haproxy_parser_test.go
git commit -m "feat: add HAProxy config data structures"
```

---

### Task 2: HAProxy config parser

**Files:**
- Modify: `internal/cmd/haproxy_parser.go` — add `ParseHAProxyConfig(text string) (*HAProxyConfig, error)`
- Modify: `internal/cmd/haproxy_parser_test.go` — add parser tests

- [ ] **Step 1: Write failing tests for the parser**

```go
// append to internal/cmd/haproxy_parser_test.go

const sampleHAProxyConfig = `
global
    log /dev/log    local0
    user haproxy
    daemon

defaults
    log    global
    mode    http
    timeout connect 5000

frontend hapctl-traefik-http
    bind *:80
    mode http
    acl host_fileserver hdr(host) -i fileserver.solutionstk.com
    use_backend hapctl-traefik-http-fileserver-backend if host_fileserver
    default_backend hapctl-traefik-http-default-backend

backend hapctl-traefik-http-fileserver-backend
    mode http
    balance roundrobin
    server hapctl-fileserver 10.99.0.142:80 check

backend hapctl-traefik-http-default-backend
    mode http
    balance roundrobin
    server hapctl-traefik-http 127.0.0.1:32080 check

listen hapctl-game-server
    bind *:7777
    mode tcp
    balance roundrobin
    server hapctl-game-server 127.0.0.1:30777 check
`

func TestParseHAProxyConfig(t *testing.T) {
    t.Run("must parse one frontend with ACLs and default_backend", func(t *testing.T) {
        cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
        require.NoError(t, err)
        require.Len(t, cfg.Frontends, 1)

        fe := cfg.Frontends[0]
        require.Equal(t, "hapctl-traefik-http", fe.Name)
        require.Equal(t, []string{"*:80"}, fe.Binds)
        require.Equal(t, "http", fe.Mode)
        require.Len(t, fe.ACLs, 1)
        require.Equal(t, "host_fileserver", fe.ACLs[0].Name)
        require.Equal(t, "hdr(host) -i fileserver.solutionstk.com", fe.ACLs[0].Condition)
        require.Len(t, fe.UseBackends, 1)
        require.Equal(t, "hapctl-traefik-http-fileserver-backend", fe.UseBackends[0].Backend)
        require.Equal(t, "host_fileserver", fe.UseBackends[0].ACLName)
        require.Equal(t, "hapctl-traefik-http-default-backend", fe.DefaultBackend)
    })

    t.Run("must parse two backends with servers", func(t *testing.T) {
        cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
        require.NoError(t, err)
        require.Len(t, cfg.Backends, 2)

        be := cfg.Backends[0]
        require.Equal(t, "hapctl-traefik-http-fileserver-backend", be.Name)
        require.Equal(t, "http", be.Mode)
        require.Equal(t, "roundrobin", be.Balance)
        require.Len(t, be.Servers, 1)
        require.Equal(t, "hapctl-fileserver", be.Servers[0].Name)
        require.Equal(t, "10.99.0.142:80", be.Servers[0].Address)
        require.Equal(t, "check", be.Servers[0].Options)
    })

    t.Run("must parse one listen block with TCP mode", func(t *testing.T) {
        cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
        require.NoError(t, err)
        require.Len(t, cfg.Listens, 1)

        ls := cfg.Listens[0]
        require.Equal(t, "hapctl-game-server", ls.Name)
        require.Equal(t, []string{"*:7777"}, ls.Binds)
        require.Equal(t, "tcp", ls.Mode)
        require.Equal(t, "roundrobin", ls.Balance)
        require.Len(t, ls.Servers, 1)
        require.Equal(t, "127.0.0.1:30777", ls.Servers[0].Address)
    })

    t.Run("must ignore global and defaults sections", func(t *testing.T) {
        cfg, err := ParseHAProxyConfig(sampleHAProxyConfig)
        require.NoError(t, err)
        // Only 1 frontend, 2 backends, 1 listen — no leakage from global/defaults
        require.Len(t, cfg.Frontends, 1)
        require.Len(t, cfg.Backends, 2)
        require.Len(t, cfg.Listens, 1)
    })

    t.Run("when empty config then returns empty HAProxyConfig", func(t *testing.T) {
        cfg, err := ParseHAProxyConfig("")
        require.NoError(t, err)
        require.Empty(t, cfg.Frontends)
        require.Empty(t, cfg.Backends)
        require.Empty(t, cfg.Listens)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run TestParseHAProxyConfig -v
```
Expected: FAIL — `ParseHAProxyConfig` undefined.

- [ ] **Step 3: Implement the parser**

Add this function to `internal/cmd/haproxy_parser.go` after the struct definitions:

```go
import (
    "strings"
)

// ParseHAProxyConfig parses raw HAProxy configuration text into an HAProxyConfig.
// The global and defaults sections are silently skipped.
func ParseHAProxyConfig(text string) (*HAProxyConfig, error) {
    cfg := &HAProxyConfig{}

    type section int
    const (
        sectionNone section = iota
        sectionIgnored
        sectionFrontend
        sectionBackend
        sectionListen
    )

    var (
        current   section
        curFE     *HAProxyFrontend
        curBE     *HAProxyBackend
        curListen *HAProxyListen
    )

    flush := func() {
        if curFE != nil {
            cfg.Frontends = append(cfg.Frontends, *curFE)
            curFE = nil
        }
        if curBE != nil {
            cfg.Backends = append(cfg.Backends, *curBE)
            curBE = nil
        }
        if curListen != nil {
            cfg.Listens = append(cfg.Listens, *curListen)
            curListen = nil
        }
    }

    for _, rawLine := range strings.Split(text, "\n") {
        line := strings.TrimSpace(rawLine)

        // Skip empty lines and comment-only lines
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        fields := strings.Fields(line)
        if len(fields) == 0 {
            continue
        }

        keyword := fields[0]

        // Detect section headers (lines that start at column 0 in raw text)
        isHeader := !strings.HasPrefix(rawLine, " ") && !strings.HasPrefix(rawLine, "\t")

        if isHeader {
            flush()
            switch keyword {
            case "global", "defaults":
                current = sectionIgnored
            case "frontend":
                current = sectionFrontend
                name := ""
                if len(fields) > 1 {
                    name = fields[1]
                }
                curFE = &HAProxyFrontend{Name: name}
            case "backend":
                current = sectionBackend
                name := ""
                if len(fields) > 1 {
                    name = fields[1]
                }
                curBE = &HAProxyBackend{Name: name}
            case "listen":
                current = sectionListen
                name := ""
                if len(fields) > 1 {
                    name = fields[1]
                }
                curListen = &HAProxyListen{Name: name}
            default:
                current = sectionIgnored
            }
            continue
        }

        if current == sectionIgnored || current == sectionNone {
            continue
        }

        switch current {
        case sectionFrontend:
            parseFrontendLine(curFE, keyword, fields)
        case sectionBackend:
            parseBackendLine(curBE, keyword, fields)
        case sectionListen:
            parseListenLine(curListen, keyword, fields)
        }
    }

    flush()
    return cfg, nil
}

func parseFrontendLine(fe *HAProxyFrontend, keyword string, fields []string) {
    switch keyword {
    case "bind":
        if len(fields) > 1 {
            fe.Binds = append(fe.Binds, fields[1])
        }
    case "mode":
        if len(fields) > 1 {
            fe.Mode = fields[1]
        }
    case "acl":
        // acl <name> <condition...>
        if len(fields) > 2 {
            fe.ACLs = append(fe.ACLs, HAProxyACL{
                Name:      fields[1],
                Condition: strings.Join(fields[2:], " "),
            })
        }
    case "use_backend":
        // use_backend <backend> if <acl_name>
        if len(fields) > 3 && fields[2] == "if" {
            fe.UseBackends = append(fe.UseBackends, HAProxyUseBackend{
                Backend: fields[1],
                ACLName: fields[3],
            })
        }
    case "default_backend":
        if len(fields) > 1 {
            fe.DefaultBackend = fields[1]
        }
    }
}

func parseBackendLine(be *HAProxyBackend, keyword string, fields []string) {
    switch keyword {
    case "mode":
        if len(fields) > 1 {
            be.Mode = fields[1]
        }
    case "balance":
        if len(fields) > 1 {
            be.Balance = fields[1]
        }
    case "server":
        // server <name> <address> [options...]
        if len(fields) >= 3 {
            be.Servers = append(be.Servers, HAProxyServer{
                Name:    fields[1],
                Address: fields[2],
                Options: strings.Join(fields[3:], " "),
            })
        }
    }
}

func parseListenLine(ls *HAProxyListen, keyword string, fields []string) {
    switch keyword {
    case "bind":
        if len(fields) > 1 {
            ls.Binds = append(ls.Binds, fields[1])
        }
    case "mode":
        if len(fields) > 1 {
            ls.Mode = fields[1]
        }
    case "balance":
        if len(fields) > 1 {
            ls.Balance = fields[1]
        }
    case "server":
        if len(fields) >= 3 {
            ls.Servers = append(ls.Servers, HAProxyServer{
                Name:    fields[1],
                Address: fields[2],
                Options: strings.Join(fields[3:], " "),
            })
        }
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run TestParseHAProxyConfig -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_parser.go internal/cmd/haproxy_parser_test.go
git commit -m "feat: implement HAProxy config parser"
```

---

### Task 3: Input reader (file path vs base64)

**Files:**
- Modify: `internal/cmd/haproxy_export.go` — create file with `readHAProxyInput` function
- Modify: `internal/cmd/haproxy_export_test.go` — create file with input reader tests

- [ ] **Step 1: Write failing tests**

```go
// internal/cmd/haproxy_export_test.go
package cmd

import (
    "encoding/base64"
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/require"
)

const minimalHAProxyCfg = `
frontend test-http
    bind *:9090
    mode http
    default_backend test-backend

backend test-backend
    mode http
    balance roundrobin
    server srv1 127.0.0.1:8080 check
`

func TestReadHAProxyInput(t *testing.T) {
    t.Run("given file path then returns file contents", func(t *testing.T) {
        dir := t.TempDir()
        p := filepath.Join(dir, "haproxy.cfg")
        require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))

        got, err := readHAProxyInput(p, "")
        require.NoError(t, err)
        require.Contains(t, got, "frontend test-http")
    })

    t.Run("given base64 string then returns decoded contents", func(t *testing.T) {
        enc := base64.StdEncoding.EncodeToString([]byte(minimalHAProxyCfg))
        got, err := readHAProxyInput("", enc)
        require.NoError(t, err)
        require.Contains(t, got, "frontend test-http")
    })

    t.Run("when both provided then file path takes precedence", func(t *testing.T) {
        dir := t.TempDir()
        p := filepath.Join(dir, "haproxy.cfg")
        require.NoError(t, os.WriteFile(p, []byte(minimalHAProxyCfg), 0644))
        enc := base64.StdEncoding.EncodeToString([]byte("different content"))

        got, err := readHAProxyInput(p, enc)
        require.NoError(t, err)
        require.Contains(t, got, "frontend test-http")
    })

    t.Run("when neither provided then returns error", func(t *testing.T) {
        _, err := readHAProxyInput("", "")
        require.Error(t, err)
    })

    t.Run("when file path does not exist then returns error", func(t *testing.T) {
        _, err := readHAProxyInput("/nonexistent/haproxy.cfg", "")
        require.Error(t, err)
    })

    t.Run("when base64 is invalid then returns error", func(t *testing.T) {
        _, err := readHAProxyInput("", "!!!not-base64!!!")
        require.Error(t, err)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run TestReadHAProxyInput -v
```
Expected: FAIL — `readHAProxyInput` undefined.

- [ ] **Step 3: Implement `readHAProxyInput`**

```go
// internal/cmd/haproxy_export.go
package cmd

import (
    "encoding/base64"
    "fmt"
    "os"
)

// readHAProxyInput returns the raw HAProxy config text from either a file path
// or a base64-encoded string. filePath takes precedence when both are provided.
func readHAProxyInput(filePath, b64 string) (string, error) {
    if filePath != "" {
        data, err := os.ReadFile(filePath)
        if err != nil {
            return "", fmt.Errorf("cannot read HAProxy config file %s: %w", filePath, err)
        }
        return string(data), nil
    }

    if b64 != "" {
        data, err := base64.StdEncoding.DecodeString(b64)
        if err != nil {
            return "", fmt.Errorf("invalid base64 input: %w", err)
        }
        return string(data), nil
    }

    return "", fmt.Errorf("provide either --file or --base64")
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run TestReadHAProxyInput -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: add HAProxy input reader (file or base64)"
```

---

### Task 4: Port conflict detection

**Files:**
- Modify: `internal/cmd/haproxy_export.go` — add `extractPort`, `collectUsedPorts`, `checkPortConflict`
- Modify: `internal/cmd/haproxy_export_test.go` — add conflict detection tests

- [ ] **Step 1: Write failing tests**

```go
// append to internal/cmd/haproxy_export_test.go

func TestExtractPort(t *testing.T) {
    t.Run("given *:80 then returns 80", func(t *testing.T) {
        p, err := extractPort("*:80")
        require.NoError(t, err)
        require.Equal(t, "80", p)
    })

    t.Run("given 10.99.0.168:3306 then returns 3306", func(t *testing.T) {
        p, err := extractPort("10.99.0.168:3306")
        require.NoError(t, err)
        require.Equal(t, "3306", p)
    })

    t.Run("given bind without colon then returns error", func(t *testing.T) {
        _, err := extractPort("noport")
        require.Error(t, err)
    })
}

func TestCheckPortConflict(t *testing.T) {
    t.Run("when port not in used set then returns false", func(t *testing.T) {
        used := map[string]struct{}{"80": {}}
        require.False(t, checkPortConflict("443", used))
    })

    t.Run("when port already in used set then returns true", func(t *testing.T) {
        used := map[string]struct{}{"80": {}}
        require.True(t, checkPortConflict("80", used))
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run "TestExtractPort|TestCheckPortConflict" -v
```
Expected: FAIL — functions undefined.

- [ ] **Step 3: Implement port utilities**

Add to `internal/cmd/haproxy_export.go`:

```go
import (
    "strings"
)

// extractPort parses the port from a HAProxy bind address (e.g. "*:80", "10.0.0.1:443").
func extractPort(bind string) (string, error) {
    idx := strings.LastIndex(bind, ":")
    if idx < 0 {
        return "", fmt.Errorf("cannot determine port from bind address %q", bind)
    }
    return bind[idx+1:], nil
}

// checkPortConflict reports whether the given port is already registered.
func checkPortConflict(port string, used map[string]struct{}) bool {
    _, exists := used[port]
    return exists
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run "TestExtractPort|TestCheckPortConflict" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: add HAProxy port conflict detection utilities"
```

---

### Task 5: HAProxy → Traefik HTTP converter

**Files:**
- Modify: `internal/cmd/haproxy_export.go` — add `convertHTTPFrontend`
- Modify: `internal/cmd/haproxy_export_test.go` — add HTTP conversion tests

The converter takes a `HAProxyFrontend`, its associated `HAProxyBackend` map, and the entrypoint name, then returns a `DynamicConfig` ready to be saved.

Conversion rules:
- Each `use_backend <be> if <acl>` → look up ACL condition to extract the host domain → create router with `Host(\`domain\`)` rule
- `default_backend <be>` → create router with rule `PathPrefix(\`/\`)` and priority 1 (lower than ACL routers)
- Backend servers → `http://<address>` URLs in `LoadBalancer.Servers`
- Entrypoint name is derived from port: port 80 → "web", port 443 → "websecure", others → frontend name

- [ ] **Step 1: Write failing tests**

```go
// append to internal/cmd/haproxy_export_test.go

func TestConvertHTTPFrontend(t *testing.T) {
    fe := HAProxyFrontend{
        Name:  "hapctl-traefik-http",
        Binds: []string{"*:80"},
        Mode:  "http",
        ACLs: []HAProxyACL{
            {Name: "host_fileserver", Condition: "hdr(host) -i fileserver.solutionstk.com"},
        },
        UseBackends: []HAProxyUseBackend{
            {Backend: "hapctl-traefik-http-fileserver-backend", ACLName: "host_fileserver"},
        },
        DefaultBackend: "hapctl-traefik-http-default-backend",
    }
    backends := map[string]HAProxyBackend{
        "hapctl-traefik-http-fileserver-backend": {
            Name:    "hapctl-traefik-http-fileserver-backend",
            Mode:    "http",
            Balance: "roundrobin",
            Servers: []HAProxyServer{{Name: "srv1", Address: "10.99.0.142:80", Options: "check"}},
        },
        "hapctl-traefik-http-default-backend": {
            Name:    "hapctl-traefik-http-default-backend",
            Mode:    "http",
            Balance: "roundrobin",
            Servers: []HAProxyServer{{Name: "srv2", Address: "127.0.0.1:32080", Options: "check"}},
        },
    }

    t.Run("must create HTTP router for ACL-based backend with Host rule", func(t *testing.T) {
        cfg := convertHTTPFrontend(fe, backends, "web")
        require.NotNil(t, cfg.HTTP)
        router, ok := cfg.HTTP.Routers["hapctl-traefik-http-fileserver-backend"]
        require.True(t, ok)
        require.Equal(t, "Host(`fileserver.solutionstk.com`)", router.Rule)
        require.Equal(t, []string{"web"}, router.EntryPoints)
        require.Equal(t, "hapctl-traefik-http-fileserver-backend", router.Service)
    })

    t.Run("must create HTTP service with correct server URL for ACL backend", func(t *testing.T) {
        cfg := convertHTTPFrontend(fe, backends, "web")
        svc, ok := cfg.HTTP.Services["hapctl-traefik-http-fileserver-backend"]
        require.True(t, ok)
        require.Len(t, svc.LoadBalancer.Servers, 1)
        require.Equal(t, "http://10.99.0.142:80", svc.LoadBalancer.Servers[0].URL)
    })

    t.Run("must create default_backend router with PathPrefix rule and lower priority", func(t *testing.T) {
        cfg := convertHTTPFrontend(fe, backends, "web")
        router, ok := cfg.HTTP.Routers["hapctl-traefik-http-default-backend"]
        require.True(t, ok)
        require.Equal(t, "PathPrefix(`/`)", router.Rule)
        require.Equal(t, 1, router.Priority)
    })

    t.Run("must create service for default_backend with correct server URL", func(t *testing.T) {
        cfg := convertHTTPFrontend(fe, backends, "web")
        svc, ok := cfg.HTTP.Services["hapctl-traefik-http-default-backend"]
        require.True(t, ok)
        require.Len(t, svc.LoadBalancer.Servers, 1)
        require.Equal(t, "http://127.0.0.1:32080", svc.LoadBalancer.Servers[0].URL)
    })
}

func TestEntrypointNameForPort(t *testing.T) {
    t.Run("port 80 returns web", func(t *testing.T) {
        require.Equal(t, "web", entrypointNameForPort("80", "any-frontend"))
    })
    t.Run("port 443 returns websecure", func(t *testing.T) {
        require.Equal(t, "websecure", entrypointNameForPort("443", "any-frontend"))
    })
    t.Run("other port returns frontend name", func(t *testing.T) {
        require.Equal(t, "hapctl-vpn-http", entrypointNameForPort("8080", "hapctl-vpn-http"))
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run "TestConvertHTTPFrontend|TestEntrypointNameForPort" -v
```
Expected: FAIL — functions undefined.

- [ ] **Step 3: Implement the HTTP converter**

Add to `internal/cmd/haproxy_export.go`:

```go
import (
    "regexp"
)

var aclHostRe = regexp.MustCompile(`(?i)hdr\(host\)\s+-i\s+(\S+)`)

// entrypointNameForPort maps common ports to Traefik standard entrypoint names.
func entrypointNameForPort(port, frontendName string) string {
    switch port {
    case "80":
        return "web"
    case "443":
        return "websecure"
    default:
        return frontendName
    }
}

// convertHTTPFrontend converts an HAProxy HTTP frontend (and its referenced backends)
// into a Traefik DynamicConfig with HTTP routers and services.
func convertHTTPFrontend(fe HAProxyFrontend, backends map[string]HAProxyBackend, entrypoint string) *DynamicConfig {
    cfg := &DynamicConfig{
        HTTP: &HTTPConfig{
            Routers:  make(map[string]*Router),
            Services: make(map[string]*Service),
        },
    }

    // Build ACL name → host domain lookup
    aclHost := make(map[string]string)
    for _, acl := range fe.ACLs {
        m := aclHostRe.FindStringSubmatch(acl.Condition)
        if len(m) > 1 {
            aclHost[acl.Name] = m[1]
        }
    }

    // Create routers for each use_backend rule
    for _, ub := range fe.UseBackends {
        host, ok := aclHost[ub.ACLName]
        if !ok {
            continue
        }

        cfg.HTTP.Routers[ub.Backend] = &Router{
            Rule:        fmt.Sprintf("Host(`%s`)", host),
            EntryPoints: []string{entrypoint},
            Service:     ub.Backend,
            Priority:    10,
        }

        if be, found := backends[ub.Backend]; found {
            cfg.HTTP.Services[ub.Backend] = buildHTTPService(be)
        }
    }

    // Create catch-all router for default_backend
    if fe.DefaultBackend != "" {
        cfg.HTTP.Routers[fe.DefaultBackend] = &Router{
            Rule:        "PathPrefix(`/`)",
            EntryPoints: []string{entrypoint},
            Service:     fe.DefaultBackend,
            Priority:    1,
        }

        if be, found := backends[fe.DefaultBackend]; found {
            cfg.HTTP.Services[fe.DefaultBackend] = buildHTTPService(be)
        }
    }

    return cfg
}

func buildHTTPService(be HAProxyBackend) *Service {
    var servers []ServerURL
    for _, srv := range be.Servers {
        servers = append(servers, ServerURL{URL: "http://" + srv.Address})
    }
    return &Service{LoadBalancer: &LoadBalancer{Servers: servers}}
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run "TestConvertHTTPFrontend|TestEntrypointNameForPort" -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: implement HAProxy HTTP frontend to Traefik converter"
```

---

### Task 6: HAProxy → Traefik TCP converter

**Files:**
- Modify: `internal/cmd/haproxy_export.go` — add `convertTCPListen`
- Modify: `internal/cmd/haproxy_export_test.go` — add TCP conversion tests

- [ ] **Step 1: Write failing tests**

```go
// append to internal/cmd/haproxy_export_test.go

func TestConvertTCPListen(t *testing.T) {
    ls := HAProxyListen{
        Name:    "hapctl-game-server",
        Binds:   []string{"*:7777"},
        Mode:    "tcp",
        Balance: "roundrobin",
        Servers: []HAProxyServer{
            {Name: "hapctl-game-server", Address: "127.0.0.1:30777", Options: "check"},
        },
    }

    t.Run("must create TCP router with HostSNI wildcard rule", func(t *testing.T) {
        cfg := convertTCPListen(ls, "hapctl-game-server")
        require.NotNil(t, cfg.TCP)
        router, ok := cfg.TCP.Routers["hapctl-game-server"]
        require.True(t, ok)
        require.Equal(t, "HostSNI(`*`)", router.Rule)
        require.Equal(t, []string{"hapctl-game-server"}, router.EntryPoints)
        require.Equal(t, "hapctl-game-server", router.Service)
    })

    t.Run("must create TCP service with server address", func(t *testing.T) {
        cfg := convertTCPListen(ls, "hapctl-game-server")
        svc, ok := cfg.TCP.Services["hapctl-game-server"]
        require.True(t, ok)
        require.Len(t, svc.LoadBalancer.Servers, 1)
        require.Equal(t, "127.0.0.1:30777", svc.LoadBalancer.Servers[0].Address)
    })

    t.Run("must include TLS passthrough for non-standard TCP", func(t *testing.T) {
        cfg := convertTCPListen(ls, "hapctl-game-server")
        router := cfg.TCP.Routers["hapctl-game-server"]
        require.NotNil(t, router.TLS)
        require.True(t, router.TLS.Passthrough)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run TestConvertTCPListen -v
```
Expected: FAIL — `convertTCPListen` undefined.

- [ ] **Step 3: Implement the TCP converter**

Add to `internal/cmd/haproxy_export.go`:

```go
// convertTCPListen converts an HAProxy listen block (TCP mode) into a Traefik
// DynamicConfig with a TCP router and service. The entrypoint name maps to the
// listen name (the user must add the corresponding entrypoint to traefik.yaml).
func convertTCPListen(ls HAProxyListen, entrypoint string) *DynamicConfig {
    cfg := &DynamicConfig{
        TCP: &TCPConfig{
            Routers:  make(map[string]*TCPRouter),
            Services: make(map[string]*TCPService),
        },
    }

    cfg.TCP.Routers[ls.Name] = &TCPRouter{
        Rule:        "HostSNI(`*`)",
        EntryPoints: []string{entrypoint},
        Service:     ls.Name,
        TLS:         &TLSConf{Passthrough: true},
    }

    var servers []ServerAddress
    for _, srv := range ls.Servers {
        servers = append(servers, ServerAddress{Address: srv.Address})
    }

    cfg.TCP.Services[ls.Name] = &TCPService{
        LoadBalancer: &TCPLoadBalancer{Servers: servers},
    }

    return cfg
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run TestConvertTCPListen -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: implement HAProxy TCP listen to Traefik converter"
```

---

### Task 7: Export orchestrator (conflict check + file writing)

**Files:**
- Modify: `internal/cmd/haproxy_export.go` — add `runHAProxyExport` orchestrator
- Modify: `internal/cmd/haproxy_export_test.go` — add orchestration tests

- [ ] **Step 1: Write failing tests**

```go
// append to internal/cmd/haproxy_export_test.go

const fullHAProxyConfig = `
frontend hapctl-traefik-http
    bind *:80
    mode http
    acl host_fileserver hdr(host) -i fileserver.solutionstk.com
    use_backend hapctl-traefik-http-fileserver-backend if host_fileserver
    default_backend hapctl-traefik-http-default-backend

backend hapctl-traefik-http-fileserver-backend
    mode http
    balance roundrobin
    server hapctl-fileserver 10.99.0.142:80 check

backend hapctl-traefik-http-default-backend
    mode http
    balance roundrobin
    server hapctl-traefik-http 127.0.0.1:32080 check

listen hapctl-game-server
    bind *:7777
    mode tcp
    balance roundrobin
    server hapctl-game-server 127.0.0.1:30777 check
`

func TestExportHAProxyToDir(t *testing.T) {
    t.Run("must create one yaml file per frontend", func(t *testing.T) {
        dir := t.TempDir()
        warnings, err := exportHAProxyToDir(fullHAProxyConfig, dir)
        require.NoError(t, err)
        require.Empty(t, warnings)

        _, err = os.Stat(filepath.Join(dir, "hapctl-traefik-http.yaml"))
        require.NoError(t, err)
    })

    t.Run("must create one yaml file per listen block", func(t *testing.T) {
        dir := t.TempDir()
        _, err := exportHAProxyToDir(fullHAProxyConfig, dir)
        require.NoError(t, err)

        _, err = os.Stat(filepath.Join(dir, "hapctl-game-server.yaml"))
        require.NoError(t, err)
    })

    t.Run("when same port used twice then second block is skipped with warning", func(t *testing.T) {
        conflictCfg := `
frontend first-http
    bind *:80
    mode http
    default_backend first-backend

backend first-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8001 check

frontend second-http
    bind *:80
    mode http
    default_backend second-backend

backend second-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8002 check
`
        dir := t.TempDir()
        warnings, err := exportHAProxyToDir(conflictCfg, dir)
        require.NoError(t, err)
        require.Len(t, warnings, 1)
        require.Contains(t, warnings[0], "80")

        _, err = os.Stat(filepath.Join(dir, "first-http.yaml"))
        require.NoError(t, err)
        _, statErr := os.Stat(filepath.Join(dir, "second-http.yaml"))
        require.True(t, os.IsNotExist(statErr))
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run TestExportHAProxyToDir -v
```
Expected: FAIL — `exportHAProxyToDir` undefined.

- [ ] **Step 3: Implement the orchestrator**

Add to `internal/cmd/haproxy_export.go`:

```go
// exportHAProxyToDir parses the given HAProxy config text and writes one Traefik
// dynamic YAML file per frontend/listen block into outDir.
// It returns a list of warning messages for skipped blocks (port conflicts).
func exportHAProxyToDir(text, outDir string) ([]string, error) {
    haCfg, err := ParseHAProxyConfig(text)
    if err != nil {
        return nil, err
    }

    // Index backends by name for fast lookup
    backendMap := make(map[string]HAProxyBackend, len(haCfg.Backends))
    for _, be := range haCfg.Backends {
        backendMap[be.Name] = be
    }

    usedPorts := make(map[string]struct{})
    var warnings []string

    // Export HTTP frontends
    for _, fe := range haCfg.Frontends {
        port, skipped := resolveBindPort(fe.Binds, fe.Name, usedPorts, &warnings)
        if skipped {
            continue
        }
        usedPorts[port] = struct{}{}

        entrypoint := entrypointNameForPort(port, fe.Name)
        dynCfg := convertHTTPFrontend(fe, backendMap, entrypoint)

        outPath := filepath.Join(outDir, fe.Name+".yaml")
        if err := saveDynamicConfig(outPath, dynCfg); err != nil {
            return warnings, err
        }
    }

    // Export TCP listen blocks
    for _, ls := range haCfg.Listens {
        port, skipped := resolveBindPort(ls.Binds, ls.Name, usedPorts, &warnings)
        if skipped {
            continue
        }
        usedPorts[port] = struct{}{}

        entrypoint := entrypointNameForPort(port, ls.Name)
        dynCfg := convertTCPListen(ls, entrypoint)

        outPath := filepath.Join(outDir, ls.Name+".yaml")
        if err := saveDynamicConfig(outPath, dynCfg); err != nil {
            return warnings, err
        }
    }

    return warnings, nil
}

// resolveBindPort extracts the port from the first bind address of a block.
// It appends to warnings and returns skipped=true if the port is already used.
func resolveBindPort(binds []string, name string, usedPorts map[string]struct{}, warnings *[]string) (port string, skipped bool) {
    if len(binds) == 0 {
        *warnings = append(*warnings, fmt.Sprintf("WARNING: %q has no bind address, skipping", name))
        return "", true
    }

    p, err := extractPort(binds[0])
    if err != nil {
        *warnings = append(*warnings, fmt.Sprintf("WARNING: %q — cannot parse bind port: %v, skipping", name, err))
        return "", true
    }

    if checkPortConflict(p, usedPorts) {
        *warnings = append(*warnings, fmt.Sprintf("WARNING: port %s already used, skipping %q", p, name))
        return "", true
    }

    return p, false
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./internal/cmd/... -run TestExportHAProxyToDir -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: add HAProxy export orchestrator with port conflict detection"
```

---

### Task 8: CLI command wiring

**Files:**
- Create: `internal/cmd/haproxy.go` — parent `haproxy` command
- Modify: `internal/cmd/haproxy_export.go` — add `haproxyExportCmd` cobra command + `init()`

- [ ] **Step 1: Write failing test for command existence**

```go
// append to internal/cmd/haproxy_export_test.go

func TestHAProxyExportCommandFlags(t *testing.T) {
    t.Run("must have --file flag", func(t *testing.T) {
        f := haproxyExportCmd.Flags().Lookup("file")
        require.NotNil(t, f)
    })

    t.Run("must have --base64 flag", func(t *testing.T) {
        f := haproxyExportCmd.Flags().Lookup("base64")
        require.NotNil(t, f)
    })

    t.Run("must have --output-dir flag", func(t *testing.T) {
        f := haproxyExportCmd.Flags().Lookup("output-dir")
        require.NotNil(t, f)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

```
go test ./internal/cmd/... -run TestHAProxyExportCommandFlags -v
```
Expected: FAIL — `haproxyExportCmd` undefined.

- [ ] **Step 3: Create the parent haproxy command**

```go
// internal/cmd/haproxy.go
package cmd

import "github.com/spf13/cobra"

var haproxyCmd = &cobra.Command{
    Use:   "haproxy",
    Short: "HAProxy integration utilities",
    Long:  "Tools for importing and exporting HAProxy configurations.",
}

func init() {
    rootCmd.AddCommand(haproxyCmd)
}
```

- [ ] **Step 4: Add the export subcommand to `haproxy_export.go`**

Add the following to the **end** of `internal/cmd/haproxy_export.go`:

```go
import (
    "path/filepath"

    "github.com/eliasmeireles/traefikctl/internal/logger"
    "github.com/spf13/cobra"
)

var (
    haproxyExportFile      string
    haproxyExportBase64    string
    haproxyExportOutputDir string
)

var haproxyExportCmd = &cobra.Command{
    Use:   "export",
    Short: "Convert an HAProxy config to Traefik dynamic YAML files",
    Long: `Read an HAProxy configuration (from a file or base64-encoded string) and
generate Traefik dynamic YAML files — one per frontend/listen block.

The global and defaults sections of the HAProxy config are ignored.
Blocks with ports that conflict with previously processed blocks are skipped
with a warning.

Examples:
  traefikctl haproxy export --file /etc/haproxy/haproxy.cfg
  traefikctl haproxy export --base64 <base64-encoded-config>
  traefikctl haproxy export --file haproxy.cfg --output-dir /tmp/traefik-dynamic`,
    SilenceUsage: true,
    RunE:         runHAProxyExport,
}

func init() {
    haproxyExportCmd.Flags().StringVar(&haproxyExportFile, "file", "", "Path to HAProxy config file")
    haproxyExportCmd.Flags().StringVar(&haproxyExportBase64, "base64", "", "Base64-encoded HAProxy config")
    haproxyExportCmd.Flags().StringVar(&haproxyExportOutputDir, "output-dir", defaultDynamicDir, "Output directory for Traefik YAML files")

    haproxyCmd.AddCommand(haproxyExportCmd)
}

func runHAProxyExport(cmd *cobra.Command, args []string) error {
    text, err := readHAProxyInput(haproxyExportFile, haproxyExportBase64)
    if err != nil {
        return err
    }

    outDir := haproxyExportOutputDir
    if !filepath.IsAbs(outDir) {
        return fmt.Errorf("output-dir must be an absolute path, got: %s", outDir)
    }

    if err := os.MkdirAll(outDir, 0755); err != nil {
        return permissionHint("create output directory", outDir, err)
    }

    warnings, err := exportHAProxyToDir(text, outDir)
    for _, w := range warnings {
        logger.Warn(w)
    }
    if err != nil {
        return err
    }

    logger.Info("HAProxy export complete. Files written to %s", outDir)
    logger.Info("NOTE: TCP entrypoints must be manually added to your traefik.yaml.")
    return nil
}
```

- [ ] **Step 5: Run all tests to verify everything passes**

```
go test ./internal/cmd/... -v
```
Expected: All tests PASS, including `TestHAProxyExportCommandFlags`.

- [ ] **Step 6: Build and verify CLI help**

```
make build && ./build/traefikctl haproxy export --help
```
Expected output shows usage, `--file`, `--base64`, and `--output-dir` flags.

- [ ] **Step 7: Commit**

```bash
git add internal/cmd/haproxy.go internal/cmd/haproxy_export.go internal/cmd/haproxy_export_test.go
git commit -m "feat: wire haproxy export CLI command"
```

---

### Task 9: Full suite verification

- [ ] **Step 1: Run the full test suite with coverage**

```
make test
```
Expected: All tests pass, no regressions.

- [ ] **Step 2: Run a smoke test with the example config**

```bash
cat > /tmp/haproxy-test.cfg << 'EOF'
global
    user haproxy
    daemon

defaults
    log    global
    mode    http
    timeout connect 5000

frontend hapctl-traefik-http
    bind *:80
    mode http
    acl host_fileserver hdr(host) -i fileserver.solutionstk.com
    use_backend hapctl-traefik-http-fileserver-backend if host_fileserver
    default_backend hapctl-traefik-http-default-backend

backend hapctl-traefik-http-fileserver-backend
    mode http
    balance roundrobin
    server hapctl-fileserver 10.99.0.142:80 check

backend hapctl-traefik-http-default-backend
    mode http
    balance roundrobin
    server hapctl-traefik-http 127.0.0.1:32080 check

listen hapctl-game-server
    bind *:7777
    mode tcp
    balance roundrobin
    server hapctl-game-server 127.0.0.1:30777 check
EOF

./build/traefikctl haproxy export --file /tmp/haproxy-test.cfg --output-dir /tmp/traefik-out
cat /tmp/traefik-out/hapctl-traefik-http.yaml
cat /tmp/traefik-out/hapctl-game-server.yaml
```

Expected: Two YAML files created with correct Traefik router/service definitions.

- [ ] **Step 3: Smoke test with base64 input**

```bash
B64=$(base64 -w0 /tmp/haproxy-test.cfg)
./build/traefikctl haproxy export --base64 "$B64" --output-dir /tmp/traefik-out2
ls /tmp/traefik-out2/
```
Expected: Same two YAML files created.

- [ ] **Step 4: Smoke test port conflict warning**

```bash
cat > /tmp/conflict.cfg << 'EOF'
frontend first-http
    bind *:80
    mode http
    default_backend first-backend
backend first-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8001 check
frontend second-http
    bind *:80
    mode http
    default_backend second-backend
backend second-backend
    mode http
    balance roundrobin
    server srv 127.0.0.1:8002 check
EOF

./build/traefikctl haproxy export --file /tmp/conflict.cfg --output-dir /tmp/traefik-conflict
ls /tmp/traefik-conflict/
```
Expected: Only `first-http.yaml` created; `[WARN]` line about port 80 conflict printed to stdout.

- [ ] **Step 5: Commit**

```bash
git add .
git commit -m "test: verify full HAProxy export feature"
```

---

### Task 10: Integration test HAProxy config fixture

**Files:**
- Create: `.dev/multipass/test-fixtures/haproxy-test.cfg` — HAProxy config that mirrors the existing docker-compose apps (app1 on 8081, app2 on 8082)
- Modify: `.dev/multipass/docker-compose.yml` — add TCP echo service for TCP routing validation

The fixture must produce routing that matches what the existing nginx containers serve so that curl assertions can validate real proxied responses.

- [ ] **Step 1: Create the test fixtures directory and HAProxy config**

```cfg
# .dev/multipass/test-fixtures/haproxy-test.cfg
# Integration test config — mirrors the docker-compose nginx apps

global
    user haproxy
    daemon

defaults
    log    global
    mode    http
    timeout connect 5000
    timeout client  50000
    timeout server  50000

# HTTP routing — maps to nginx-app1 and nginx-app2 in docker-compose
frontend hapctl-test-http
    bind *:80
    mode http
    acl host_app1 hdr(host) -i app1.localhost
    acl host_app2 hdr(host) -i app2.localhost
    use_backend hapctl-test-app1-backend if host_app1
    use_backend hapctl-test-app2-backend if host_app2
    default_backend hapctl-test-app1-backend

backend hapctl-test-app1-backend
    mode http
    balance roundrobin
    server app1 127.0.0.1:8081 check

backend hapctl-test-app2-backend
    mode http
    balance roundrobin
    server app2 127.0.0.1:8082 check

# Duplicate port — must trigger warning and be skipped
frontend hapctl-test-http-dup
    bind *:80
    mode http
    default_backend hapctl-test-dup-backend

backend hapctl-test-dup-backend
    mode http
    balance roundrobin
    server dup 127.0.0.1:9999 check

# TCP echo service (socat echo on port 7001, proxied via Traefik on 7000)
listen hapctl-test-tcp
    bind *:7000
    mode tcp
    balance roundrobin
    server tcp-echo 127.0.0.1:7001 check
```

- [ ] **Step 2: Add socat TCP echo service to docker-compose**

Replace the contents of `.dev/multipass/docker-compose.yml`:

```yaml
services:
  nginx-app1:
    image: nginx:alpine
    container_name: nginx-app1
    ports:
      - "8081:80"
    volumes:
      - ./html/app1:/usr/share/nginx/html:ro
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped

  nginx-app2:
    image: nginx:alpine
    container_name: nginx-app2
    ports:
      - "8082:80"
    volumes:
      - ./html/app2:/usr/share/nginx/html:ro
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped

  tcp-echo:
    image: alpine:latest
    container_name: tcp-echo
    command: sh -c "apk add --no-cache socat && socat TCP-LISTEN:7001,fork,reuseaddr EXEC:'/bin/cat'"
    ports:
      - "7001:7001"
    restart: unless-stopped
```

- [ ] **Step 3: Copy fixture to volumes so multipass mount picks it up**

Add to `.dev/multipass/setup.sh` inside `prepare_volume()` (after the existing `cp` lines):

```bash
mkdir -p "$VOLUMES_DIR/test-fixtures"
cp -r "$SCRIPT_DIR/test-fixtures/"* "$VOLUMES_DIR/test-fixtures/"
```

- [ ] **Step 4: Commit**

```bash
git add .dev/multipass/test-fixtures/haproxy-test.cfg .dev/multipass/docker-compose.yml .dev/multipass/setup.sh
git commit -m "test: add HAProxy export integration test fixtures"
```

---

### Task 11: Integration test script (runs inside the VM)

**Files:**
- Create: `.dev/multipass/test-haproxy-export.sh` — self-contained test runner executed inside the multipass VM

This script runs as root inside the VM. It:
1. Exports the fixture HAProxy config to a temp directory
2. Validates the generated YAML files exist and contain expected keys
3. Deploys the generated YAML to `/etc/traefik/dynamic/` and reloads Traefik
4. Uses `curl` to validate that Traefik is correctly proxying to app1 and app2
5. Validates that the duplicate-port frontend was skipped (no YAML file, warning printed)
6. Validates the TCP config YAML structure (since TCP entrypoints require manual `traefik.yaml` changes, full TCP proxy test is out of scope)
7. Validates base64 input produces the same output as file input

- [ ] **Step 1: Create the test script**

```bash
#!/bin/bash
# .dev/multipass/test-haproxy-export.sh
# Runs INSIDE the multipass VM as root.
# Prerequisites: traefikctl installed, Traefik running, docker-compose apps up.

set -euo pipefail

PASS=0
FAIL=0
FIXTURE="/home/ubuntu/traefikctl/test-fixtures/haproxy-test.cfg"
OUT_DIR="/tmp/haproxy-export-test"
DYN_DIR="/etc/traefik/dynamic"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL=$((FAIL+1)); }

assert_file_exists() {
    local file="$1" label="$2"
    if [ -f "$file" ]; then pass "$label"; else fail "$label — file not found: $file"; fi
}

assert_file_missing() {
    local file="$1" label="$2"
    if [ ! -f "$file" ]; then pass "$label"; else fail "$label — file should not exist: $file"; fi
}

assert_contains() {
    local file="$1" pattern="$2" label="$3"
    if grep -q "$pattern" "$file" 2>/dev/null; then pass "$label"; else fail "$label — pattern '$pattern' not found in $file"; fi
}

assert_http_status() {
    local url="$1" header="$2" expected_status="$3" label="$4"
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" -H "$header" "$url" 2>/dev/null || echo "000")
    if [ "$status" = "$expected_status" ]; then
        pass "$label (HTTP $status)"
    else
        fail "$label — expected HTTP $expected_status, got $status"
    fi
}

assert_response_contains() {
    local url="$1" header="$2" pattern="$3" label="$4"
    local body
    body=$(curl -s -H "$header" "$url" 2>/dev/null || echo "")
    if echo "$body" | grep -q "$pattern"; then
        pass "$label"
    else
        fail "$label — pattern '$pattern' not found in response body"
    fi
}

echo ""
echo "========================================"
echo " HAProxy Export Integration Tests"
echo "========================================"
echo ""

# ── Section 1: Export from file ───────────────────────────────────────────────
echo "--- Section 1: Export from file ---"

rm -rf "$OUT_DIR"
EXPORT_OUTPUT=$(traefikctl haproxy export --file "$FIXTURE" --output-dir "$OUT_DIR" 2>&1)

assert_file_exists "$OUT_DIR/hapctl-test-http.yaml" "HTTP frontend YAML is created"
assert_file_missing "$OUT_DIR/hapctl-test-http-dup.yaml" "Duplicate-port frontend is NOT created"
assert_file_exists "$OUT_DIR/hapctl-test-tcp.yaml" "TCP listen YAML is created"

# ── Section 2: Validate YAML content ─────────────────────────────────────────
echo ""
echo "--- Section 2: YAML content validation ---"

assert_contains "$OUT_DIR/hapctl-test-http.yaml" "Host(\`app1.localhost\`)" "HTTP YAML has app1 Host rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "Host(\`app2.localhost\`)" "HTTP YAML has app2 Host rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "PathPrefix(\`/\`)" "HTTP YAML has default PathPrefix rule"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "127.0.0.1:8081" "HTTP YAML has app1 backend address"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "127.0.0.1:8082" "HTTP YAML has app2 backend address"
assert_contains "$OUT_DIR/hapctl-test-http.yaml" "web" "HTTP YAML references 'web' entrypoint for port 80"

assert_contains "$OUT_DIR/hapctl-test-tcp.yaml" "HostSNI" "TCP YAML has HostSNI rule"
assert_contains "$OUT_DIR/hapctl-test-tcp.yaml" "127.0.0.1:7001" "TCP YAML has correct backend address"
assert_contains "$OUT_DIR/hapctl-test-tcp.yaml" "passthrough" "TCP YAML has TLS passthrough"

# ── Section 3: Port conflict warning ─────────────────────────────────────────
echo ""
echo "--- Section 3: Port conflict warning ---"

if echo "$EXPORT_OUTPUT" | grep -qi "port 80.*skip\|skip.*port 80\|already used"; then
    pass "Warning printed for duplicate port 80"
else
    fail "No warning found for duplicate port 80 — output was: $EXPORT_OUTPUT"
fi

# ── Section 4: Base64 input produces identical output ─────────────────────────
echo ""
echo "--- Section 4: Base64 input ---"

OUT_DIR_B64="/tmp/haproxy-export-test-b64"
rm -rf "$OUT_DIR_B64"
B64=$(base64 -w0 "$FIXTURE")
traefikctl haproxy export --base64 "$B64" --output-dir "$OUT_DIR_B64" > /dev/null 2>&1

assert_file_exists "$OUT_DIR_B64/hapctl-test-http.yaml" "base64 input creates HTTP frontend YAML"
assert_file_missing "$OUT_DIR_B64/hapctl-test-http-dup.yaml" "base64 input also skips duplicate-port frontend"
assert_file_exists "$OUT_DIR_B64/hapctl-test-tcp.yaml" "base64 input creates TCP YAML"

# Compare the two outputs are identical (excluding timestamps if any)
if diff -q "$OUT_DIR/hapctl-test-http.yaml" "$OUT_DIR_B64/hapctl-test-http.yaml" > /dev/null 2>&1; then
    pass "base64 output matches file input output"
else
    fail "base64 output differs from file input output"
fi

# ── Section 5: Deploy to Traefik and validate HTTP proxy ──────────────────────
echo ""
echo "--- Section 5: Live Traefik proxy routing ---"

# Back up existing dynamic configs and deploy exported ones
BACKUP_DIR="/tmp/traefik-dynamic-backup-$(date +%s)"
cp -r "$DYN_DIR" "$BACKUP_DIR" 2>/dev/null || true

# Replace dynamic dir contents with exported config
rm -f "$DYN_DIR"/*.yaml 2>/dev/null || true
cp "$OUT_DIR/hapctl-test-http.yaml" "$DYN_DIR/"

# Traefik watches for file changes — wait for reload
sleep 3

# Verify Traefik is still running
if systemctl is-active traefikctl > /dev/null 2>&1; then
    pass "Traefik service is still active after config deploy"
else
    fail "Traefik service is not active after config deploy"
fi

# Test routing via Host header
assert_http_status "http://127.0.0.1" "Host: app1.localhost" "200" "app1.localhost routes to app1 (HTTP 200)"
assert_http_status "http://127.0.0.1" "Host: app2.localhost" "200" "app2.localhost routes to app2 (HTTP 200)"
assert_http_status "http://127.0.0.1" "Host: unknown.localhost" "200" "default backend serves catch-all (HTTP 200)"

# Verify responses come from different backends
APP1_BODY=$(curl -s -H "Host: app1.localhost" "http://127.0.0.1" 2>/dev/null || echo "")
APP2_BODY=$(curl -s -H "Host: app2.localhost" "http://127.0.0.1" 2>/dev/null || echo "")

if [ "$APP1_BODY" != "$APP2_BODY" ]; then
    pass "app1 and app2 return different responses (routing is working)"
else
    fail "app1 and app2 return identical responses — routing may not be working"
fi

# ── Restore original dynamic config ──────────────────────────────────────────
rm -f "$DYN_DIR"/*.yaml 2>/dev/null || true
cp "$BACKUP_DIR"/*.yaml "$DYN_DIR/" 2>/dev/null || true
systemctl reload traefikctl 2>/dev/null || true
sleep 2

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "========================================"
echo " Results: ${PASS} passed, ${FAIL} failed"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x .dev/multipass/test-haproxy-export.sh
```

- [ ] **Step 3: Add it to the volume preparation in setup.sh**

In `.dev/multipass/setup.sh`, inside `prepare_volume()`, after the existing `cp` lines add:

```bash
cp "$SCRIPT_DIR/test-haproxy-export.sh" "$VOLUMES_DIR/"
```

- [ ] **Step 4: Commit**

```bash
git add .dev/multipass/test-haproxy-export.sh .dev/multipass/setup.sh
git commit -m "test: add HAProxy export integration test script"
```

---

### Task 12: Host-side e2e runner and Makefile target

**Files:**
- Create: `.dev/multipass/run-e2e-tests.sh` — host-side script that builds, syncs, and runs tests via `multipass exec`
- Modify: `Makefile` — add `test-e2e` target

The host-side script handles building the binary, syncing it to the VM, starting docker-compose services if needed, and running the test script via `multipass exec`. This lets CI or a developer run `make test-e2e` to execute the full integration suite.

- [ ] **Step 1: Create the host-side runner**

```bash
#!/bin/bash
# .dev/multipass/run-e2e-tests.sh
# Run from the project root. Requires multipass VM 'traefikctl-dev' to be running.

set -euo pipefail

VM_NAME="traefikctl-dev"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VOLUMES_DIR="${SCRIPT_DIR}/.volumes"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== HAProxy Export E2E Tests ==="
echo ""

# Check VM is running
if ! multipass list | grep -q "${VM_NAME}.*Running"; then
    echo "[ERROR] VM '${VM_NAME}' is not running."
    echo "Start it with: cd .dev/multipass && bash setup.sh"
    exit 1
fi

# Build latest binary
echo "[1/5] Building traefikctl binary..."
cd "$PROJECT_ROOT"
make build
echo "[OK] Build complete"

# Install updated binary in VM
echo "[2/5] Installing binary in VM..."
multipass transfer "$PROJECT_ROOT/build/traefikctl" "$VM_NAME:/tmp/traefikctl"
multipass exec "$VM_NAME" -- sudo mv /tmp/traefikctl /usr/local/bin/traefikctl
multipass exec "$VM_NAME" -- sudo chmod +x /usr/local/bin/traefikctl
echo "[OK] Binary installed"

# Sync test fixtures and test script to the volume
echo "[3/5] Syncing test fixtures..."
mkdir -p "$VOLUMES_DIR/test-fixtures"
cp -r "$SCRIPT_DIR/test-fixtures/"* "$VOLUMES_DIR/test-fixtures/"
cp "$SCRIPT_DIR/test-haproxy-export.sh" "$VOLUMES_DIR/"
echo "[OK] Fixtures synced to mounted volume"

# Ensure docker-compose services are up
echo "[4/5] Ensuring docker-compose services are up..."
multipass exec "$VM_NAME" -- bash -c "cd /home/ubuntu/traefikctl && docker compose up -d --wait"
echo "[OK] Services are healthy"

# Run the integration tests inside the VM
echo "[5/5] Running integration tests inside VM..."
echo ""
multipass exec "$VM_NAME" -- sudo bash /home/ubuntu/traefikctl/test-haproxy-export.sh
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x .dev/multipass/run-e2e-tests.sh
```

- [ ] **Step 3: Add `test-e2e` to the Makefile**

Read `Makefile` first, then add after the `test` target:

```makefile
test-e2e: build ## Run end-to-end integration tests via multipass (requires VM to be running)
	@bash .dev/multipass/run-e2e-tests.sh
```

The full Makefile `test-e2e` block must be placed after the existing `test:` target and follow the existing tab-indented recipe style.

- [ ] **Step 4: Run e2e tests to verify everything passes**

Ensure the multipass VM is running (`multipass list`), then:

```bash
make test-e2e
```

Expected output ends with:
```
========================================
 Results: 22 passed, 0 failed
========================================
```

- [ ] **Step 5: Commit**

```bash
git add .dev/multipass/run-e2e-tests.sh Makefile
git commit -m "test: add make test-e2e target for multipass integration tests"
```

---

## Self-Review: Spec Coverage

| Requirement | Covered by Task |
|---|---|
| Export HTTP frontend + backends as Traefik HTTP routers/services | Task 5 |
| Export TCP listen blocks as Traefik TCP routers/services | Task 6 |
| ACL `hdr(host) -i domain` → `Host(...)` rule | Task 5 |
| `default_backend` → `PathPrefix(/)` catch-all | Task 5 |
| Input from file path (`--file`) | Task 3 |
| Input from base64 (`--base64`) | Task 3 |
| Skip `global` and `defaults` sections entirely | Task 2 |
| Warn and skip on duplicate port | Tasks 4 + 7 |
| Write one YAML file per block | Task 7 |
| `traefikctl haproxy export` CLI command | Task 8 |
| Note about TCP entrypoints needing manual addition | Task 8 |
| Integration test fixture with real nginx containers | Task 10 |
| TCP echo service in docker-compose for TCP YAML validation | Task 10 |
| Test script: YAML content assertions | Task 11 |
| Test script: port conflict warning assertion | Task 11 |
| Test script: base64 vs file input parity check | Task 11 |
| Test script: live Traefik proxy routing validation via curl | Task 11 |
| Test script: backup/restore dynamic configs after test | Task 11 |
| `make test-e2e` host runner with build + sync + execute | Task 12 |
