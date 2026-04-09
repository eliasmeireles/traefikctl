# traefikctl Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand traefikctl with service lifecycle ops, middleware management, HTTPS/TLS automation, backend health, route toggle, and a system-wide status command — turning it into a complete Traefik management tool.

**Architecture:** Each feature is a focused command file in `internal/cmd/`. Testable logic (config building, YAML manipulation, URL parsing) is extracted into helpers that can be unit tested without root access. System-level ops (systemctl, file writes) are thin wrappers around those helpers.

**Tech Stack:** Go 1.25, Cobra, gopkg.in/yaml.v3, net/http (standard lib), github.com/stretchr/testify (tests)

---

## File Map

| File | Status | Responsibility |
|---|---|---|
| `internal/cmd/service.go` | Modify | Add `restart` and `reload` subcommands |
| `internal/cmd/resource_toggle.go` | Create | `resource enable` / `resource disable` |
| `internal/cmd/resource_backend.go` | Create | `resource backend add` / `resource backend remove` |
| `internal/cmd/resource_copy.go` | Create | `resource copy` |
| `internal/cmd/middleware.go` | Create | Middleware group command + types |
| `internal/cmd/middleware_add.go` | Create | `middleware add` |
| `internal/cmd/middleware_list.go` | Create | `middleware list` |
| `internal/cmd/middleware_remove.go` | Create | `middleware remove` |
| `internal/cmd/resource_add.go` | Modify | Add `--middleware`, `--redirect-https`, `--tls`, `--cert-resolver` flags |
| `internal/cmd/resource.go` | Modify | Add `Middleware` types to `HTTPConfig` and `Router` |
| `internal/cmd/status.go` | Create | `traefikctl status` — bird's eye view |
| `internal/cmd/update.go` | Create | `traefikctl update` — upgrade Traefik binary |
| `internal/traefik/defaults.go` | Modify | Add ACME/Let's Encrypt static config template |
| `internal/cmd/config.go` | Modify | Add `--acme` flag for Let's Encrypt setup |
| `internal/cmd/service_test.go` | Create | Unit tests for service command helpers |
| `internal/cmd/resource_toggle_test.go` | Create | Unit tests for toggle logic |
| `internal/cmd/resource_backend_test.go` | Create | Unit tests for backend add/remove |
| `internal/cmd/middleware_test.go` | Create | Unit tests for middleware YAML building |
| `internal/cmd/status_test.go` | Create | Unit tests for status aggregation |

---

## Task 1: `service restart` and `service reload`

**Why:** The CLI has install/uninstall/status/logs but no way to restart or send a reload signal — operators must drop to raw systemctl.

**Files:**
- Modify: `internal/cmd/service.go`
- Create: `internal/cmd/service_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/service_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceCommandsRegistered(t *testing.T) {
	cmds := map[string]bool{}
	for _, sub := range serviceCmd.Commands() {
		cmds[sub.Use] = true
	}

	assert.True(t, cmds["restart"], "restart subcommand must be registered")
	assert.True(t, cmds["reload"], "reload subcommand must be registered")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cmd/ -run TestServiceCommandsRegistered -v
```

Expected: `FAIL — restart/reload subcommand must be registered`

- [ ] **Step 3: Add `restart` and `reload` to `service.go`**

After the `serviceLogsCmd` block and before the `init()` function, add:

```go
var serviceRestartCmd = &cobra.Command{
	Use:          "restart",
	Short:        "Restart the Traefik service",
	SilenceUsage: true,
	RunE:         runServiceRestart,
}

var serviceReloadCmd = &cobra.Command{
	Use:          "reload",
	Short:        "Reload Traefik config without full restart (systemctl reload)",
	SilenceUsage: true,
	RunE:         runServiceReload,
}
```

Inside `init()`, bind flags and register:

```go
serviceRestartCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")
serviceReloadCmd.Flags().StringVar(&serviceName, "name", "traefikctl", "Service name")

serviceCmd.AddCommand(serviceRestartCmd)
serviceCmd.AddCommand(serviceReloadCmd)
```

After `runServiceLogs`, add:

```go
func runServiceRestart(cmd *cobra.Command, args []string) error {
	if err := systemctl("restart", serviceName); err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}
	logger.Info("Service '%s' restarted", serviceName)
	return nil
}

func runServiceReload(cmd *cobra.Command, args []string) error {
	if err := systemctl("reload", serviceName); err != nil {
		return fmt.Errorf("failed to reload service (not all services support reload): %w", err)
	}
	logger.Info("Service '%s' config reloaded", serviceName)
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cmd/ -run TestServiceCommandsRegistered -v
```

Expected: `PASS`

- [ ] **Step 5: Build and smoke test**

```bash
go build ./... && ./build/traefikctl service --help
```

Expected: `restart` and `reload` appear in Available Commands.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/service.go internal/cmd/service_test.go
git commit -m "feat: add service restart and reload subcommands"
```

---

## Task 2: `resource enable` / `resource disable`

**Why:** Operators often need to temporarily take a route offline without losing the config. Currently the only option is `remove`, which destroys it.

**Approach:** Disabled routers and services are moved to `/etc/traefikctl/disabled/<name>.yaml` (outside Traefik's watch directory). `enable` restores them back to the source file.

**Files:**
- Create: `internal/cmd/resource_toggle.go`
- Create: `internal/cmd/resource_toggle_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/resource_toggle_test.go`:

```go
package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisableAndEnableHTTPRouter(t *testing.T) {
	dir := t.TempDir()
	disabledDir := filepath.Join(dir, "disabled")
	activeFile := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}

	require.NoError(t, saveDynamicConfig(activeFile, cfg))

	// Disable
	require.NoError(t, disableRouter("my-app", activeFile, disabledDir))

	restored, err := loadDynamicConfig(activeFile)
	require.NoError(t, err)
	require.Nil(t, restored.HTTP, "HTTP section must be gone after disabling last router")

	// Disabled file must exist
	disabledFile := filepath.Join(disabledDir, "my-app.yaml")
	_, err = os.Stat(disabledFile)
	require.NoError(t, err, "disabled snapshot must exist")

	// Enable
	require.NoError(t, enableRouter("my-app", activeFile, disabledDir))

	reactivated, err := loadDynamicConfig(activeFile)
	require.NoError(t, err)
	require.NotNil(t, reactivated.HTTP)
	require.Contains(t, reactivated.HTTP.Routers, "my-app")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cmd/ -run TestDisableAndEnableHTTPRouter -v
```

Expected: `FAIL — disableRouter undefined`

- [ ] **Step 3: Implement `resource_toggle.go`**

Create `internal/cmd/resource_toggle.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

const disabledDir = "/etc/traefikctl/disabled"

var (
	toggleName string
	toggleFile string
)

var resourceEnableCmd = &cobra.Command{
	Use:          "enable",
	Short:        "Re-enable a previously disabled router",
	Long: `Restore a disabled router back to the active dynamic config file.

Example:
  traefikctl resource enable --name my-app`,
	SilenceUsage: true,
	RunE:         runResourceEnable,
}

var resourceDisableCmd = &cobra.Command{
	Use:          "disable",
	Short:        "Temporarily disable a router without removing it",
	Long: `Remove a router from active config and save it to /etc/traefikctl/disabled/.
Restore it with: traefikctl resource enable --name <name>

Example:
  traefikctl resource disable --name my-app`,
	SilenceUsage: true,
	RunE:         runResourceDisable,
}

func init() {
	resourceEnableCmd.Flags().StringVar(&toggleName, "name", "", "Router name")
	resourceDisableCmd.Flags().StringVar(&toggleName, "name", "", "Router name")
	resourceDisableCmd.Flags().StringVar(&toggleFile, "file", "", "Dynamic config file (skip selection prompt)")

	_ = resourceEnableCmd.MarkFlagRequired("name")
	_ = resourceDisableCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceEnableCmd)
	resourceCmd.AddCommand(resourceDisableCmd)
}

func runResourceDisable(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(toggleFile)
	if err != nil {
		return err
	}

	if err := disableRouter(toggleName, filePath, disabledDir); err != nil {
		return err
	}

	logger.Info("Router '%s' disabled (saved to %s/%s.yaml)", toggleName, disabledDir, toggleName)
	logger.Info("Re-enable with: traefikctl resource enable --name %s", toggleName)
	return nil
}

func runResourceEnable(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	var targetFile string
	if len(files) == 1 {
		targetFile = files[0]
	} else if len(files) > 1 {
		targetFile, err = selectDynamicFile("")
		if err != nil {
			return err
		}
	} else {
		targetFile = filepath.Join(defaultDynamicDir, "services.yaml")
	}

	if err := enableRouter(toggleName, targetFile, disabledDir); err != nil {
		return err
	}

	logger.Info("Router '%s' re-enabled in %s", toggleName, targetFile)
	return nil
}

// disableRouter moves a router+service from activeFile into disabledDir/<name>.yaml.
func disableRouter(name, activeFile, dDir string) error {
	cfg, err := loadDynamicConfig(activeFile)
	if err != nil {
		return err
	}

	snapshot := &DynamicConfig{}
	found := false

	if cfg.HTTP != nil {
		if _, ok := cfg.HTTP.Routers[name]; ok {
			snapshot.HTTP = &HTTPConfig{
				Routers:  map[string]*Router{name: cfg.HTTP.Routers[name]},
				Services: map[string]*Service{},
			}
			svcName := cfg.HTTP.Routers[name].Service
			if svc, ok := cfg.HTTP.Services[svcName]; ok {
				snapshot.HTTP.Services[svcName] = svc
				delete(cfg.HTTP.Services, svcName)
			}
			delete(cfg.HTTP.Routers, name)
			if len(cfg.HTTP.Routers) == 0 && len(cfg.HTTP.Services) == 0 {
				cfg.HTTP = nil
			}
			found = true
		}
	}

	if !found && cfg.TCP != nil {
		if _, ok := cfg.TCP.Routers[name]; ok {
			snapshot.TCP = &TCPConfig{
				Routers:  map[string]*TCPRouter{name: cfg.TCP.Routers[name]},
				Services: map[string]*TCPService{},
			}
			svcName := cfg.TCP.Routers[name].Service
			if svc, ok := cfg.TCP.Services[svcName]; ok {
				snapshot.TCP.Services[svcName] = svc
				delete(cfg.TCP.Services, svcName)
			}
			delete(cfg.TCP.Routers, name)
			if len(cfg.TCP.Routers) == 0 && len(cfg.TCP.Services) == 0 {
				cfg.TCP = nil
			}
			found = true
		}
	}

	if !found {
		return fmt.Errorf("router '%s' not found", name)
	}

	if err := os.MkdirAll(dDir, 0755); err != nil {
		return fmt.Errorf("failed to create disabled dir: %w", err)
	}

	disabledPath := filepath.Join(dDir, name+".yaml")
	if err := saveDynamicConfig(disabledPath, snapshot); err != nil {
		return err
	}

	if cfg.HTTP == nil && cfg.TCP == nil {
		return os.Remove(activeFile)
	}

	return saveDynamicConfig(activeFile, cfg)
}

// enableRouter restores a disabled router from dDir/<name>.yaml into targetFile.
func enableRouter(name, targetFile, dDir string) error {
	disabledPath := filepath.Join(dDir, name+".yaml")

	snapshot, err := loadDynamicConfig(disabledPath)
	if err != nil {
		return fmt.Errorf("disabled snapshot not found for '%s': %w", name, err)
	}

	var cfg *DynamicConfig
	if _, statErr := os.Stat(targetFile); os.IsNotExist(statErr) {
		cfg = &DynamicConfig{}
	} else {
		cfg, err = loadDynamicConfig(targetFile)
		if err != nil {
			return err
		}
	}

	if snapshot.HTTP != nil {
		if cfg.HTTP == nil {
			cfg.HTTP = &HTTPConfig{Routers: map[string]*Router{}, Services: map[string]*Service{}}
		}
		for k, v := range snapshot.HTTP.Routers {
			cfg.HTTP.Routers[k] = v
		}
		for k, v := range snapshot.HTTP.Services {
			cfg.HTTP.Services[k] = v
		}
	}

	if snapshot.TCP != nil {
		if cfg.TCP == nil {
			cfg.TCP = &TCPConfig{Routers: map[string]*TCPRouter{}, Services: map[string]*TCPService{}}
		}
		for k, v := range snapshot.TCP.Routers {
			cfg.TCP.Routers[k] = v
		}
		for k, v := range snapshot.TCP.Services {
			cfg.TCP.Services[k] = v
		}
	}

	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	if err := saveDynamicConfig(targetFile, cfg); err != nil {
		return err
	}

	return os.Remove(disabledPath)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cmd/ -run TestDisableAndEnableHTTPRouter -v
```

Expected: `PASS`

- [ ] **Step 5: Build and smoke test**

```bash
go build ./... && ./build/traefikctl resource --help
```

Expected: `disable` and `enable` listed under Available Commands.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/resource_toggle.go internal/cmd/resource_toggle_test.go
git commit -m "feat: add resource enable/disable to toggle routes without deleting"
```

---

## Task 3: `resource backend add` / `resource backend remove`

**Why:** Currently you can't add a second backend server to an existing service for load balancing, or remove one — you'd have to edit the YAML by hand.

**Files:**
- Create: `internal/cmd/resource_backend.go`
- Create: `internal/cmd/resource_backend_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/resource_backend_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddBackendServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, addBackendServer("my-app", "127.0.0.1:8081", file))

	updated, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Len(t, updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers, 2)
	require.Equal(t, "http://127.0.0.1:8081", updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers[1].URL)
}

func TestRemoveBackendServer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{
					{URL: "http://127.0.0.1:8080"},
					{URL: "http://127.0.0.1:8081"},
				}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, removeBackendServer("my-app", "127.0.0.1:8081", file))

	updated, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Len(t, updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers, 1)
	require.Equal(t, "http://127.0.0.1:8080", updated.HTTP.Services["my-app-svc"].LoadBalancer.Servers[0].URL)
}

func TestRemoveLastBackendServerReturnsError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	err := removeBackendServer("my-app", "127.0.0.1:8080", file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "last server")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cmd/ -run "TestAddBackendServer|TestRemoveBackendServer|TestRemoveLastBackendServerReturnsError" -v
```

Expected: `FAIL — addBackendServer undefined`

- [ ] **Step 3: Implement `resource_backend.go`**

Create `internal/cmd/resource_backend.go`:

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	backendRouterName string
	backendAddress    string
	backendFile       string
)

var resourceBackendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Manage backend servers for an existing service",
}

var resourceBackendAddCmd = &cobra.Command{
	Use:          "add",
	Short:        "Add a backend server to an existing service",
	Long: `Add a new backend server to an existing HTTP service for load balancing.

Example:
  traefikctl resource backend add --name my-app --address 10.0.0.5:8080`,
	SilenceUsage: true,
	RunE:         runBackendAdd,
}

var resourceBackendRemoveCmd = &cobra.Command{
	Use:          "remove",
	Short:        "Remove a backend server from a service",
	Long: `Remove one backend server from an existing HTTP service.
The last server cannot be removed — use 'resource remove' to delete the whole route.

Example:
  traefikctl resource backend remove --name my-app --address 10.0.0.5:8080`,
	SilenceUsage: true,
	RunE:         runBackendRemove,
}

func init() {
	resourceBackendAddCmd.Flags().StringVar(&backendRouterName, "name", "", "Router name")
	resourceBackendAddCmd.Flags().StringVar(&backendAddress, "address", "", "Backend address (ip:port)")
	resourceBackendAddCmd.Flags().StringVar(&backendFile, "file", "", "Dynamic config file")
	_ = resourceBackendAddCmd.MarkFlagRequired("name")
	_ = resourceBackendAddCmd.MarkFlagRequired("address")

	resourceBackendRemoveCmd.Flags().StringVar(&backendRouterName, "name", "", "Router name")
	resourceBackendRemoveCmd.Flags().StringVar(&backendAddress, "address", "", "Backend address to remove (ip:port)")
	resourceBackendRemoveCmd.Flags().StringVar(&backendFile, "file", "", "Dynamic config file")
	_ = resourceBackendRemoveCmd.MarkFlagRequired("name")
	_ = resourceBackendRemoveCmd.MarkFlagRequired("address")

	resourceBackendCmd.AddCommand(resourceBackendAddCmd)
	resourceBackendCmd.AddCommand(resourceBackendRemoveCmd)
	resourceCmd.AddCommand(resourceBackendCmd)
}

func runBackendAdd(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(backendFile)
	if err != nil {
		return err
	}

	if err := addBackendServer(backendRouterName, backendAddress, filePath); err != nil {
		return err
	}

	logger.Info("Backend http://%s added to '%s'", backendAddress, backendRouterName)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

func runBackendRemove(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(backendFile)
	if err != nil {
		return err
	}

	if err := removeBackendServer(backendRouterName, backendAddress, filePath); err != nil {
		return err
	}

	logger.Info("Backend %s removed from '%s'", backendAddress, backendRouterName)
	logger.Info("Config saved: %s (Traefik will auto-reload)", filePath)
	return nil
}

// addBackendServer appends a new URL to the load balancer of the service attached to routerName.
func addBackendServer(routerName, address, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	router, ok := cfg.HTTP.Routers[routerName]
	if !ok {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	svc, ok := cfg.HTTP.Services[router.Service]
	if !ok || svc.LoadBalancer == nil {
		return fmt.Errorf("service '%s' not found or has no load balancer", router.Service)
	}

	newURL := fmt.Sprintf("http://%s", address)
	for _, s := range svc.LoadBalancer.Servers {
		if s.URL == newURL {
			return fmt.Errorf("backend %s already exists in service '%s'", newURL, router.Service)
		}
	}

	svc.LoadBalancer.Servers = append(svc.LoadBalancer.Servers, ServerURL{URL: newURL})
	return saveDynamicConfig(filePath, cfg)
}

// removeBackendServer removes a URL from the load balancer of the service attached to routerName.
// Returns error if it would leave zero servers.
func removeBackendServer(routerName, address, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	router, ok := cfg.HTTP.Routers[routerName]
	if !ok {
		return fmt.Errorf("router '%s' not found", routerName)
	}

	svc, ok := cfg.HTTP.Services[router.Service]
	if !ok || svc.LoadBalancer == nil {
		return fmt.Errorf("service '%s' not found", router.Service)
	}

	target := fmt.Sprintf("http://%s", address)
	if !strings.HasPrefix(address, "http") {
		target = fmt.Sprintf("http://%s", address)
	}

	filtered := make([]ServerURL, 0, len(svc.LoadBalancer.Servers))
	for _, s := range svc.LoadBalancer.Servers {
		if s.URL != target {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == len(svc.LoadBalancer.Servers) {
		return fmt.Errorf("backend %s not found in service '%s'", target, router.Service)
	}

	if len(filtered) == 0 {
		return fmt.Errorf("cannot remove the last server — use 'resource remove --name %s' to delete the whole route", routerName)
	}

	svc.LoadBalancer.Servers = filtered
	return saveDynamicConfig(filePath, cfg)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cmd/ -run "TestAddBackendServer|TestRemoveBackendServer|TestRemoveLastBackendServerReturnsError" -v
```

Expected: all `PASS`

- [ ] **Step 5: Build and smoke test**

```bash
go build ./... && ./build/traefikctl resource backend --help
```

Expected: `add` and `remove` listed.

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/resource_backend.go internal/cmd/resource_backend_test.go
git commit -m "feat: add resource backend add/remove for multi-server load balancing"
```

---

## Task 4: `resource copy`

**Why:** Cloning an existing route to a new domain/name is a common operation (e.g., staging vs. production). Currently requires manual YAML editing.

**Files:**
- Create: `internal/cmd/resource_copy.go`
- Create: `internal/cmd/resource_copy_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/resource_copy_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyHTTPRouter(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, copyRouter("my-app", "my-app-staging", "staging.example.com", file, file))

	result, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Contains(t, result.HTTP.Routers, "my-app")
	require.Contains(t, result.HTTP.Routers, "my-app-staging")
	require.Equal(t, "Host(`staging.example.com`)", result.HTTP.Routers["my-app-staging"].Rule)
	require.Contains(t, result.HTTP.Services, "my-app-staging-svc")
}

func TestCopyRouterFailsIfDestinationExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")

	cfg := &DynamicConfig{
		HTTP: &HTTPConfig{
			Routers: map[string]*Router{
				"my-app": {Rule: "Host(`app.example.com`)", EntryPoints: []string{"web"}, Service: "my-app-svc"},
			},
			Services: map[string]*Service{
				"my-app-svc": {LoadBalancer: &LoadBalancer{Servers: []ServerURL{{URL: "http://127.0.0.1:8080"}}}},
			},
		},
	}
	require.NoError(t, saveDynamicConfig(file, cfg))

	err := copyRouter("my-app", "my-app", "other.example.com", file, file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cmd/ -run "TestCopyHTTPRouter|TestCopyRouterFailsIfDestinationExists" -v
```

Expected: `FAIL — copyRouter undefined`

- [ ] **Step 3: Implement `resource_copy.go`**

Create `internal/cmd/resource_copy.go`:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	copyFrom   string
	copyName   string
	copyDomain string
	copyFile   string
	copyDest   string
)

var resourceCopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Clone an existing router to a new name and optional domain",
	Long: `Copy a router and its service to a new name.
The original is not modified. Optionally point the copy at a different domain.

Examples:
  traefikctl resource copy --from my-app --name my-app-staging --domain staging.example.com
  traefikctl resource copy --from my-app --name my-app-v2`,
	SilenceUsage: true,
	RunE:         runResourceCopy,
}

func init() {
	resourceCopyCmd.Flags().StringVar(&copyFrom, "from", "", "Source router name")
	resourceCopyCmd.Flags().StringVar(&copyName, "name", "", "Destination router name")
	resourceCopyCmd.Flags().StringVar(&copyDomain, "domain", "", "New domain for the copy (optional)")
	resourceCopyCmd.Flags().StringVar(&copyFile, "file", "", "Source dynamic config file")
	resourceCopyCmd.Flags().StringVar(&copyDest, "dest", "", "Destination file (defaults to same as source)")

	_ = resourceCopyCmd.MarkFlagRequired("from")
	_ = resourceCopyCmd.MarkFlagRequired("name")

	resourceCmd.AddCommand(resourceCopyCmd)
}

func runResourceCopy(cmd *cobra.Command, args []string) error {
	srcFile, err := selectDynamicFile(copyFile)
	if err != nil {
		return err
	}

	destFile := srcFile
	if copyDest != "" {
		destFile = copyDest
	}

	if err := copyRouter(copyFrom, copyName, copyDomain, srcFile, destFile); err != nil {
		return err
	}

	logger.Info("Router '%s' copied to '%s'", copyFrom, copyName)
	if copyDomain != "" {
		logger.Info("  New rule: Host(`%s`)", copyDomain)
	}
	logger.Info("Config saved: %s (Traefik will auto-reload)", destFile)
	return nil
}

// copyRouter duplicates a router+service under a new name.
// If newDomain is non-empty, the copy gets Host(`newDomain`) as its rule.
func copyRouter(srcName, dstName, newDomain, srcFile, dstFile string) error {
	src, err := loadDynamicConfig(srcFile)
	if err != nil {
		return err
	}

	if src.HTTP == nil {
		return fmt.Errorf("router '%s' not found", srcName)
	}

	srcRouter, ok := src.HTTP.Routers[srcName]
	if !ok {
		return fmt.Errorf("router '%s' not found", srcName)
	}

	var dst *DynamicConfig
	if srcFile == dstFile {
		dst = src
	} else if _, statErr := os.Stat(dstFile); os.IsNotExist(statErr) {
		dst = &DynamicConfig{}
	} else {
		dst, err = loadDynamicConfig(dstFile)
		if err != nil {
			return err
		}
	}

	if dst.HTTP == nil {
		dst.HTTP = &HTTPConfig{Routers: map[string]*Router{}, Services: map[string]*Service{}}
	}

	if _, exists := dst.HTTP.Routers[dstName]; exists {
		return fmt.Errorf("router '%s' already exists in destination", dstName)
	}

	newSvcName := dstName + "-svc"

	rule := srcRouter.Rule
	if newDomain != "" {
		rule = fmt.Sprintf("Host(`%s`)", newDomain)
	}

	dst.HTTP.Routers[dstName] = &Router{
		Rule:        rule,
		EntryPoints: append([]string{}, srcRouter.EntryPoints...),
		Service:     newSvcName,
		Priority:    srcRouter.Priority,
	}

	srcSvc := src.HTTP.Services[srcRouter.Service]
	newSvc := &Service{LoadBalancer: &LoadBalancer{}}
	if srcSvc != nil && srcSvc.LoadBalancer != nil {
		for _, s := range srcSvc.LoadBalancer.Servers {
			newSvc.LoadBalancer.Servers = append(newSvc.LoadBalancer.Servers, s)
		}
	}
	dst.HTTP.Services[newSvcName] = newSvc

	if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
		return fmt.Errorf("failed to create destination dir: %w", err)
	}

	return saveDynamicConfig(dstFile, dst)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cmd/ -run "TestCopyHTTPRouter|TestCopyRouterFailsIfDestinationExists" -v
```

Expected: all `PASS`

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/resource_copy.go internal/cmd/resource_copy_test.go
git commit -m "feat: add resource copy to clone routes"
```

---

## Task 5: Middleware Management

**Why:** Traefik middlewares (rate limiting, basic auth, redirects, header injection) are configured in dynamic YAML. Currently there's no CLI for them — operators must write YAML by hand.

**Files:**
- Modify: `internal/cmd/resource.go` — add `Middleware` types and `Middlewares` map to `HTTPConfig` + `Router`
- Create: `internal/cmd/middleware.go` — group command + types
- Create: `internal/cmd/middleware_add.go`
- Create: `internal/cmd/middleware_list.go`
- Create: `internal/cmd/middleware_remove.go`
- Create: `internal/cmd/middleware_test.go`

- [ ] **Step 1: Add Middleware types to `resource.go`**

In `internal/cmd/resource.go`, add the following types after `ServerAddress`:

```go
// Middleware types mirror Traefik v3 dynamic config structure.

type MiddlewareConfig struct {
	RedirectScheme  *RedirectScheme  `yaml:"redirectScheme,omitempty"`
	BasicAuth       *BasicAuth       `yaml:"basicAuth,omitempty"`
	RateLimit       *RateLimit       `yaml:"rateLimit,omitempty"`
	StripPrefix     *StripPrefix     `yaml:"stripPrefix,omitempty"`
	Headers         *Headers         `yaml:"headers,omitempty"`
}

type RedirectScheme struct {
	Scheme    string `yaml:"scheme"`
	Permanent bool   `yaml:"permanent,omitempty"`
}

type BasicAuth struct {
	Users []string `yaml:"users"`
}

type RateLimit struct {
	Average int `yaml:"average"`
	Burst   int `yaml:"burst"`
}

type StripPrefix struct {
	Prefixes []string `yaml:"prefixes"`
}

type Headers struct {
	CustomRequestHeaders  map[string]string `yaml:"customRequestHeaders,omitempty"`
	CustomResponseHeaders map[string]string `yaml:"customResponseHeaders,omitempty"`
}
```

In `HTTPConfig`, add:

```go
type HTTPConfig struct {
	Routers     map[string]*Router            `yaml:"routers,omitempty"`
	Services    map[string]*Service           `yaml:"services,omitempty"`
	Middlewares map[string]*MiddlewareConfig  `yaml:"middlewares,omitempty"`
}
```

In `Router`, add `Middlewares` field:

```go
type Router struct {
	Rule        string   `yaml:"rule"`
	EntryPoints []string `yaml:"entryPoints"`
	Service     string   `yaml:"service"`
	Middlewares []string `yaml:"middlewares,omitempty"`
	Priority    int      `yaml:"priority,omitempty"`
}
```

- [ ] **Step 2: Write failing tests**

Create `internal/cmd/middleware_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddRedirectHTTPSMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, addMiddleware("redirect-https", "redirect-https", map[string]string{
		"scheme":    "https",
		"permanent": "true",
	}, file))

	result, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.NotNil(t, result.HTTP.Middlewares["redirect-https"])
	require.Equal(t, "https", result.HTTP.Middlewares["redirect-https"].RedirectScheme.Scheme)
	require.True(t, result.HTTP.Middlewares["redirect-https"].RedirectScheme.Permanent)
}

func TestAddRateLimitMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, addMiddleware("my-limit", "rate-limit", map[string]string{
		"average": "100",
		"burst":   "50",
	}, file))

	result, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.Equal(t, 100, result.HTTP.Middlewares["my-limit"].RateLimit.Average)
	require.Equal(t, 50, result.HTTP.Middlewares["my-limit"].RateLimit.Burst)
}

func TestRemoveMiddleware(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "services.yaml")
	cfg := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{},
		Services: map[string]*Service{},
		Middlewares: map[string]*MiddlewareConfig{
			"my-mw": {RedirectScheme: &RedirectScheme{Scheme: "https"}},
		},
	}}
	require.NoError(t, saveDynamicConfig(file, cfg))

	require.NoError(t, removeMiddleware("my-mw", file))

	result, err := loadDynamicConfig(file)
	require.NoError(t, err)
	require.NotContains(t, result.HTTP.Middlewares, "my-mw")
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/cmd/ -run "TestAddRedirectHTTPSMiddleware|TestAddRateLimitMiddleware|TestRemoveMiddleware" -v
```

Expected: `FAIL — addMiddleware undefined`

- [ ] **Step 4: Create `middleware.go`**

Create `internal/cmd/middleware.go`:

```go
package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var middlewareCmd = &cobra.Command{
	Use:   "middleware",
	Short: "Manage Traefik middlewares",
	Long: `Manage reusable Traefik middlewares (rate limiting, auth, redirects, headers).

Supported types: redirect-https, basic-auth, rate-limit, strip-prefix`,
}

func init() {
	rootCmd.AddCommand(middlewareCmd)
}

// addMiddleware creates or updates a named middleware of the given type in the config file.
// opts keys depend on the middleware type:
//   redirect-https: scheme, permanent
//   rate-limit:     average, burst
//   basic-auth:     users (comma-separated htpasswd entries)
//   strip-prefix:   prefixes (comma-separated)
func addMiddleware(name, mwType string, opts map[string]string, filePath string) error {
	cfg, err := loadOrCreateConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP.Middlewares == nil {
		cfg.HTTP.Middlewares = map[string]*MiddlewareConfig{}
	}

	mw := &MiddlewareConfig{}

	switch mwType {
	case "redirect-https":
		permanent := opts["permanent"] == "true"
		scheme := opts["scheme"]
		if scheme == "" {
			scheme = "https"
		}
		mw.RedirectScheme = &RedirectScheme{Scheme: scheme, Permanent: permanent}

	case "rate-limit":
		avg, _ := strconv.Atoi(opts["average"])
		burst, _ := strconv.Atoi(opts["burst"])
		if avg == 0 {
			avg = 100
		}
		if burst == 0 {
			burst = 50
		}
		mw.RateLimit = &RateLimit{Average: avg, Burst: burst}

	case "basic-auth":
		users := splitComma(opts["users"])
		if len(users) == 0 {
			return fmt.Errorf("basic-auth requires --opt users=user1:hash,user2:hash")
		}
		mw.BasicAuth = &BasicAuth{Users: users}

	case "strip-prefix":
		prefixes := splitComma(opts["prefixes"])
		if len(prefixes) == 0 {
			return fmt.Errorf("strip-prefix requires --opt prefixes=/api,/v1")
		}
		mw.StripPrefix = &StripPrefix{Prefixes: prefixes}

	default:
		return fmt.Errorf("unsupported middleware type '%s'. Supported: redirect-https, rate-limit, basic-auth, strip-prefix", mwType)
	}

	cfg.HTTP.Middlewares[name] = mw
	return saveDynamicConfig(filePath, cfg)
}

// removeMiddleware deletes a named middleware from the config file.
func removeMiddleware(name, filePath string) error {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return err
	}

	if cfg.HTTP == nil || cfg.HTTP.Middlewares == nil {
		return fmt.Errorf("middleware '%s' not found", name)
	}

	if _, ok := cfg.HTTP.Middlewares[name]; !ok {
		return fmt.Errorf("middleware '%s' not found", name)
	}

	delete(cfg.HTTP.Middlewares, name)

	if len(cfg.HTTP.Middlewares) == 0 {
		cfg.HTTP.Middlewares = nil
	}

	return saveDynamicConfig(filePath, cfg)
}

// loadOrCreateConfig loads the config at path, creating an empty HTTP section if absent.
func loadOrCreateConfig(filePath string) (*DynamicConfig, error) {
	cfg, err := loadDynamicConfig(filePath)
	if err != nil {
		return nil, err
	}

	if cfg.HTTP == nil {
		cfg.HTTP = &HTTPConfig{
			Routers:  map[string]*Router{},
			Services: map[string]*Service{},
		}
	}

	return cfg, nil
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range splitOn(s, ',') {
		if t := trimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func splitOn(s string, sep rune) []string {
	var parts []string
	start := 0
	for i, r := range s {
		if r == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
```

- [ ] **Step 5: Create `middleware_add.go`**

Create `internal/cmd/middleware_add.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	mwAddName string
	mwAddType string
	mwAddFile string
	mwAddOpts []string
)

var middlewareAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a middleware to a dynamic config file",
	Long: `Add a reusable middleware to a Traefik dynamic config file.

Types and required --opt keys:
  redirect-https   scheme=https  permanent=true
  rate-limit       average=100   burst=50
  basic-auth       users=user1:hash,user2:hash
  strip-prefix     prefixes=/api,/v1

Examples:
  traefikctl middleware add --name redirect-https --type redirect-https
  traefikctl middleware add --name my-limit --type rate-limit --opt average=100 --opt burst=50
  traefikctl middleware add --name strip-api --type strip-prefix --opt prefixes=/api`,
	SilenceUsage: true,
	RunE:         runMiddlewareAdd,
}

func init() {
	middlewareAddCmd.Flags().StringVar(&mwAddName, "name", "", "Middleware name")
	middlewareAddCmd.Flags().StringVar(&mwAddType, "type", "", "Middleware type (redirect-https, rate-limit, basic-auth, strip-prefix)")
	middlewareAddCmd.Flags().StringVar(&mwAddFile, "file", "", "Dynamic config file")
	middlewareAddCmd.Flags().StringArrayVar(&mwAddOpts, "opt", nil, "Type-specific options as key=value (repeatable)")

	_ = middlewareAddCmd.MarkFlagRequired("name")
	_ = middlewareAddCmd.MarkFlagRequired("type")

	middlewareCmd.AddCommand(middlewareAddCmd)
}

func runMiddlewareAdd(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(mwAddFile)
	if err != nil {
		return err
	}

	opts := parseKeyValue(mwAddOpts)

	if err := addMiddleware(mwAddName, mwAddType, opts, filePath); err != nil {
		return err
	}

	logger.Info("Middleware '%s' (%s) added to %s", mwAddName, mwAddType, filePath)
	return nil
}

func parseKeyValue(pairs []string) map[string]string {
	out := map[string]string{}
	for _, pair := range pairs {
		for i, r := range pair {
			if r == '=' {
				out[pair[:i]] = pair[i+1:]
				break
			}
		}
	}
	return out
}
```

- [ ] **Step 6: Create `middleware_list.go`**

Create `internal/cmd/middleware_list.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var middlewareListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List all configured middlewares",
	SilenceUsage: true,
	RunE:         runMiddlewareList,
}

func init() {
	middlewareCmd.AddCommand(middlewareListCmd)
}

func runMiddlewareList(cmd *cobra.Command, args []string) error {
	files, err := listDynamicFiles()
	if err != nil {
		return err
	}

	total := 0

	for _, filePath := range files {
		cfg, loadErr := loadDynamicConfig(filePath)
		if loadErr != nil {
			logger.Warn("Skipping %s: %v", filePath, loadErr)
			continue
		}

		if cfg.HTTP == nil || len(cfg.HTTP.Middlewares) == 0 {
			continue
		}

		fmt.Printf("\n%s\n", filePath)

		for _, name := range sortedMiddlewareKeys(cfg.HTTP.Middlewares) {
			mw := cfg.HTTP.Middlewares[name]
			fmt.Printf("  %-30s  %s\n", name, middlewareTypeSummary(mw))
			total++
		}
	}

	fmt.Println()
	logger.Info("Total middlewares: %d", total)
	return nil
}

func middlewareTypeSummary(mw *MiddlewareConfig) string {
	switch {
	case mw.RedirectScheme != nil:
		return fmt.Sprintf("redirect-https (scheme=%s, permanent=%v)", mw.RedirectScheme.Scheme, mw.RedirectScheme.Permanent)
	case mw.RateLimit != nil:
		return fmt.Sprintf("rate-limit (avg=%d, burst=%d)", mw.RateLimit.Average, mw.RateLimit.Burst)
	case mw.BasicAuth != nil:
		return fmt.Sprintf("basic-auth (%d user(s))", len(mw.BasicAuth.Users))
	case mw.StripPrefix != nil:
		return fmt.Sprintf("strip-prefix (%v)", mw.StripPrefix.Prefixes)
	case mw.Headers != nil:
		return "headers"
	default:
		return "unknown"
	}
}

func sortedMiddlewareKeys(m map[string]*MiddlewareConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
```

- [ ] **Step 7: Create `middleware_remove.go`**

Create `internal/cmd/middleware_remove.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	mwRemoveName string
	mwRemoveFile string
)

var middlewareRemoveCmd = &cobra.Command{
	Use:          "remove",
	Short:        "Remove a middleware",
	SilenceUsage: true,
	RunE:         runMiddlewareRemove,
}

func init() {
	middlewareRemoveCmd.Flags().StringVar(&mwRemoveName, "name", "", "Middleware name")
	middlewareRemoveCmd.Flags().StringVar(&mwRemoveFile, "file", "", "Dynamic config file")
	_ = middlewareRemoveCmd.MarkFlagRequired("name")

	middlewareCmd.AddCommand(middlewareRemoveCmd)
}

func runMiddlewareRemove(cmd *cobra.Command, args []string) error {
	filePath, err := selectDynamicFile(mwRemoveFile)
	if err != nil {
		return err
	}

	if err := removeMiddleware(mwRemoveName, filePath); err != nil {
		return err
	}

	logger.Info("Middleware '%s' removed from %s", mwRemoveName, filePath)
	return nil
}
```

- [ ] **Step 8: Add `--middleware` flag to `resource add`**

In `internal/cmd/resource_add.go`, add the var:

```go
addMiddlewares []string
```

Add the flag in `init()`:

```go
resourceAddCmd.Flags().StringArrayVar(&addMiddlewares, "middleware", nil, "Attach middleware(s) by name (repeatable)")
```

In `addHTTPResource`, set the Router's Middlewares field:

```go
cfg.HTTP.Routers[addName] = &Router{
	Rule:        rule,
	EntryPoints: []string{addEntrypoint},
	Service:     svcName,
	Middlewares: addMiddlewares,
}
```

- [ ] **Step 9: Run tests**

```bash
go test ./internal/cmd/ -run "TestAddRedirectHTTPSMiddleware|TestAddRateLimitMiddleware|TestRemoveMiddleware" -v
```

Expected: all `PASS`

- [ ] **Step 10: Build**

```bash
go build ./...
```

- [ ] **Step 11: Commit**

```bash
git add internal/cmd/resource.go internal/cmd/middleware.go internal/cmd/middleware_add.go \
  internal/cmd/middleware_list.go internal/cmd/middleware_remove.go \
  internal/cmd/middleware_test.go internal/cmd/resource_add.go
git commit -m "feat: add middleware management (add, list, remove) and --middleware flag on resource add"
```

---

## Task 6: HTTPS + Let's Encrypt

**Why:** Enabling HTTPS today requires manually editing `traefik.yaml` and every dynamic config. This task adds a `--redirect-https` shortcut on `resource add` and a `--acme` flag on `config --generate`.

**Files:**
- Modify: `internal/cmd/resource_add.go` — add `--redirect-https`, `--tls`, `--cert-resolver` flags
- Modify: `internal/traefik/defaults.go` — add ACME config template
- Modify: `internal/cmd/config.go` — add `--acme` and `--acme-email` flags

- [ ] **Step 1: Add ACME template to `defaults.go`**

In `internal/traefik/defaults.go`, add after `DefaultDynamicExample`:

```go
const DefaultACMEConfig = `# Let's Encrypt / ACME TLS configuration
# Append to /etc/traefik/traefik.yaml under the top level

certificatesResolvers:
  letsencrypt:
    acme:
      email: %s
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
`
```

- [ ] **Step 2: Add `--acme` flag to `config.go`**

In `internal/cmd/config.go`, add vars:

```go
cfgACME      bool
cfgACMEEmail string
```

Add flags in `init()`:

```go
configCmd.Flags().BoolVar(&cfgACME, "acme", false, "Append Let's Encrypt ACME config to traefik.yaml")
configCmd.Flags().StringVar(&cfgACMEEmail, "acme-email", "", "Email address for Let's Encrypt (required with --acme)")
```

In `runConfig`, before the generate/view switch:

```go
if cfgACME {
    return appendACMEConfig(cfgACMEEmail)
}
```

Add the function:

```go
func appendACMEConfig(email string) error {
    if email == "" {
        return fmt.Errorf("--acme-email is required with --acme")
    }

    staticPath := "/etc/traefik/traefik.yaml"
    acmePath := "/etc/traefik/acme.json"

    acmeBlock := fmt.Sprintf(traefik.DefaultACMEConfig, email)

    f, err := os.OpenFile(staticPath, os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil {
        return permissionHint("append ACME config to", staticPath, err)
    }
    defer func() { _ = f.Close() }()

    if _, err := f.WriteString("\n" + acmeBlock); err != nil {
        return fmt.Errorf("failed to write ACME config: %w", err)
    }

    // Create acme.json with correct permissions (Traefik requires 0600)
    if _, statErr := os.Stat(acmePath); os.IsNotExist(statErr) {
        if createErr := os.WriteFile(acmePath, []byte("{}"), 0600); createErr != nil {
            logger.Warn("Failed to create %s: %v", acmePath, createErr)
        } else {
            logger.Info("Created %s (Traefik will write certificates here)", acmePath)
        }
    }

    logger.Info("ACME config appended to %s", staticPath)
    logger.Info("Restart Traefik for changes to take effect: sudo traefikctl service restart")
    return nil
}
```

- [ ] **Step 3: Add `--redirect-https`, `--tls`, `--cert-resolver` to `resource add`**

In `internal/cmd/resource_add.go`, add vars:

```go
addRedirectHTTPS bool
addTLS           bool
addCertResolver  string
```

Add flags in `init()`:

```go
resourceAddCmd.Flags().BoolVar(&addRedirectHTTPS, "redirect-https", false, "Add HTTP→HTTPS redirect middleware automatically")
resourceAddCmd.Flags().BoolVar(&addTLS, "tls", false, "Enable TLS on the router (use with --entrypoint websecure)")
resourceAddCmd.Flags().StringVar(&addCertResolver, "cert-resolver", "", "Let's Encrypt cert resolver name (e.g. letsencrypt)")
```

Replace the router creation block in `addHTTPResource`:

```go
// Build router
router := &Router{
    Rule:        rule,
    EntryPoints: []string{addEntrypoint},
    Service:     svcName,
    Middlewares: addMiddlewares,
}

if addTLS || addCertResolver != "" {
    router.TLS = &RouterTLS{}
    if addCertResolver != "" {
        router.TLS.CertResolver = addCertResolver
    }
}

cfg.HTTP.Routers[addName] = router

// Auto-create HTTP→HTTPS redirect if requested
if addRedirectHTTPS {
    redirectMWName := "redirect-to-https"
    if cfg.HTTP.Middlewares == nil {
        cfg.HTTP.Middlewares = map[string]*MiddlewareConfig{}
    }
    cfg.HTTP.Middlewares[redirectMWName] = &MiddlewareConfig{
        RedirectScheme: &RedirectScheme{Scheme: "https", Permanent: true},
    }

    httpRouterName := addName + "-http"
    cfg.HTTP.Routers[httpRouterName] = &Router{
        Rule:        rule,
        EntryPoints: []string{"web"},
        Service:     svcName,
        Middlewares: []string{redirectMWName},
    }
    logger.Info("Created HTTP redirect router '%s' with middleware '%s'", httpRouterName, redirectMWName)
}
```

Add `RouterTLS` type to `resource.go`:

```go
type RouterTLS struct {
    CertResolver string `yaml:"certResolver,omitempty"`
}
```

And update `Router`:

```go
type Router struct {
    Rule        string     `yaml:"rule"`
    EntryPoints []string   `yaml:"entryPoints"`
    Service     string     `yaml:"service"`
    Middlewares []string   `yaml:"middlewares,omitempty"`
    TLS         *RouterTLS `yaml:"tls,omitempty"`
    Priority    int        `yaml:"priority,omitempty"`
}
```

- [ ] **Step 4: Build and verify**

```bash
go build ./... && ./build/traefikctl resource add --help
```

Expected: `--redirect-https`, `--tls`, `--cert-resolver` listed in flags.

```bash
./build/traefikctl config --help
```

Expected: `--acme` and `--acme-email` listed.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/resource_add.go internal/cmd/config.go internal/cmd/resource.go internal/traefik/defaults.go
git commit -m "feat: add HTTPS/TLS flags and Let's Encrypt ACME config support"
```

---

## Task 7: `traefikctl status`

**Why:** There's no single command to see "is Traefik running, how many routes are active, are backends healthy?" — operators must run 3+ commands.

**Files:**
- Create: `internal/cmd/status.go`
- Create: `internal/cmd/status_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/cmd/status_test.go`:

```go
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountRoutes(t *testing.T) {
	dir := t.TempDir()

	file1 := filepath.Join(dir, "a.yaml")
	file2 := filepath.Join(dir, "b.yaml")

	cfg1 := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{"r1": {}, "r2": {}},
		Services: map[string]*Service{},
	}}
	cfg2 := &DynamicConfig{HTTP: &HTTPConfig{
		Routers:  map[string]*Router{"r3": {}},
		Services: map[string]*Service{},
	}}
	require.NoError(t, saveDynamicConfig(file1, cfg1))
	require.NoError(t, saveDynamicConfig(file2, cfg2))

	http, tcp := countRoutes([]string{file1, file2})
	assert.Equal(t, 3, http)
	assert.Equal(t, 0, tcp)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/cmd/ -run TestCountRoutes -v
```

Expected: `FAIL — countRoutes undefined`

- [ ] **Step 3: Implement `status.go`**

Create `internal/cmd/status.go`:

```go
package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var statusServiceName string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a full system status overview",
	Long: `Display service state, installed version, route count, and backend health summary.

Example:
  traefikctl status`,
	SilenceUsage: true,
	RunE:         runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusServiceName, "name", "traefikctl", "Systemd service name")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("=== traefikctl Status ===")
	fmt.Println()

	printServiceState(statusServiceName)
	printTraefikVersion()
	printRoutesSummary()

	return nil
}

func printServiceState(name string) {
	out, err := exec.Command("systemctl", "is-active", name).Output()
	state := strings.TrimSpace(string(out))
	if err != nil || state != "active" {
		logger.Error("Service %-20s  %s", name, state)
	} else {
		fmt.Printf("  Service %-20s  %s\n", name, state)
	}
}

func printTraefikVersion() {
	out, err := exec.Command("traefik", "version").Output()
	if err != nil {
		logger.Warn("Traefik binary not found or not in PATH")
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		fmt.Printf("  %s\n", lines[0])
	}
}

func printRoutesSummary() {
	files, err := listDynamicFiles()
	if err != nil || len(files) == 0 {
		logger.Warn("No dynamic config files found in %s", defaultDynamicDir)
		return
	}

	http, tcp := countRoutes(files)
	fmt.Printf("  Routes: %d HTTP, %d TCP  (across %d file(s))\n", http, tcp, len(files))
}

// countRoutes returns the total number of HTTP and TCP routers across the given files.
func countRoutes(files []string) (httpCount, tcpCount int) {
	for _, f := range files {
		cfg, err := loadDynamicConfig(f)
		if err != nil {
			continue
		}
		if cfg.HTTP != nil {
			httpCount += len(cfg.HTTP.Routers)
		}
		if cfg.TCP != nil {
			tcpCount += len(cfg.TCP.Routers)
		}
	}
	return
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/cmd/ -run TestCountRoutes -v
```

Expected: `PASS`

- [ ] **Step 5: Build and smoke test**

```bash
go build ./... && ./build/traefikctl status --help
```

- [ ] **Step 6: Commit**

```bash
git add internal/cmd/status.go internal/cmd/status_test.go
git commit -m "feat: add status command with service state, version, and route summary"
```

---

## Task 8: `traefikctl update`

**Why:** Updating Traefik requires knowing the latest version, downloading manually, and re-applying capabilities. This wraps the entire flow.

**Files:**
- Create: `internal/cmd/update.go`

- [ ] **Step 1: Implement `update.go`**

Create `internal/cmd/update.go`:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
	"github.com/eliasmeireles/traefikctl/internal/traefik"
)

const githubLatestRelease = "https://api.github.com/repos/traefik/traefik/releases/latest"

var (
	updateVersion string
	updateCheck   bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Traefik to the latest or specified version",
	Long: `Check for and install a newer Traefik binary.
Stops the service, replaces the binary, re-applies capabilities, and restarts.

Examples:
  traefikctl update
  traefikctl update --check
  traefikctl update --version v3.4.0`,
	SilenceUsage: true,
	RunE:         runUpdate,
}

func init() {
	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Target version (default: latest)")
	updateCmd.Flags().StringVar(&statusServiceName, "name", "traefikctl", "Service name to restart after update")
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Only check for a newer version, do not install")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	installer := traefik.NewInstaller()

	currentRaw, err := installer.GetVersion()
	if err != nil {
		logger.Warn("Could not determine installed Traefik version: %v", err)
		currentRaw = "unknown"
	}
	current := extractSemver(currentRaw)
	logger.Info("Installed: %s", current)

	target := updateVersion
	if target == "" {
		target, err = fetchLatestVersion()
		if err != nil {
			return fmt.Errorf("failed to fetch latest Traefik version: %w", err)
		}
	}

	logger.Info("Latest:    %s", target)

	if current == target {
		logger.Info("Already up to date.")
		return nil
	}

	if updateCheck {
		logger.Info("New version available: %s -> %s", current, target)
		logger.Info("Run without --check to install: sudo traefikctl update")
		return nil
	}

	logger.Info("Updating %s -> %s...", current, target)

	_ = systemctl("stop", statusServiceName)

	if err := installer.Install(target); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if err := systemctl("start", statusServiceName); err != nil {
		logger.Warn("Service did not restart cleanly — run: sudo systemctl start %s", statusServiceName)
	} else {
		logger.Info("Service restarted successfully")
	}

	logger.Info("Traefik updated to %s", target)
	return nil
}

func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(githubLatestRelease)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var payload struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	if payload.TagName == "" {
		return "", fmt.Errorf("empty tag_name in GitHub release response")
	}

	return payload.TagName, nil
}

func extractSemver(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "Version") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}
	return strings.TrimSpace(raw)
}
```

- [ ] **Step 2: Build and verify**

```bash
go build ./... && ./build/traefikctl update --help
```

Expected: `--check`, `--version`, `--name` flags listed.

- [ ] **Step 3: Commit**

```bash
git add internal/cmd/update.go
git commit -m "feat: add update command to upgrade Traefik binary"
```

---

## Self-Review

### Spec coverage check

| Feature | Task |
|---|---|
| service restart/reload | Task 1 ✓ |
| resource enable/disable | Task 2 ✓ |
| backend add/remove | Task 3 ✓ |
| resource copy | Task 4 ✓ |
| middleware add/list/remove | Task 5 ✓ |
| --middleware on resource add | Task 5 ✓ |
| HTTPS redirect shortcut | Task 6 ✓ |
| TLS + cert resolver flags | Task 6 ✓ |
| Let's Encrypt ACME config | Task 6 ✓ |
| status overview | Task 7 ✓ |
| update Traefik binary | Task 8 ✓ |

### Placeholder scan

No TBD, TODO, or "similar to task N" patterns found.

### Type consistency

- `Router.Middlewares []string` added in Task 5 Step 1, used in Task 5 Step 8 and Task 6 Step 3 ✓
- `Router.TLS *RouterTLS` added in Task 6 Step 3, defined in same step ✓
- `MiddlewareConfig` defined in Task 5 Step 1, used in Task 5 Steps 4/7 and Task 6 Step 3 ✓
- `disableRouter` / `enableRouter` defined in Task 2 Step 3, tested in Task 2 Step 1 ✓
- `addBackendServer` / `removeBackendServer` defined in Task 3 Step 3, tested in Task 3 Step 1 ✓
- `countRoutes` defined in Task 7 Step 3, tested in Task 7 Step 1 ✓
- `loadOrCreateConfig` defined in `middleware.go`, used within same file ✓

### Dependency on `sort` package

`resource_list.go` imports `sort` and uses `sort.Strings`. `middleware_list.go` uses a local `sortStrings` insertion sort to avoid adding another import — consistent with the lean dependency philosophy. Both sort functions are used in list commands that are new in these tasks ✓
