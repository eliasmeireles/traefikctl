package cmd

import "strings"

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

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		keyword := fields[0]
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
		if len(fields) > 2 {
			fe.ACLs = append(fe.ACLs, HAProxyACL{
				Name:      fields[1],
				Condition: strings.Join(fields[2:], " "),
			})
		}
	case "use_backend":
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
