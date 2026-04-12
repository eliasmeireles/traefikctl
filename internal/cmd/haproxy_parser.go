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
	Name      string
	Condition string
}

// HAProxyUseBackend represents a conditional backend selection rule.
type HAProxyUseBackend struct {
	Backend string
	ACLName string
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
	Name    string
	Address string
	Options string
}
