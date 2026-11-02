package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestHostRule(t *testing.T) {
	for _, tc := range []struct{ title, domains, protocol, want string }{
		{"must_default_to_wildcard", "", "tcp", "HostSNI(`*`)"},
		{"must_accept_explicit_wildcard", "*", "tcp", "HostSNI(`*`)"},
		{"must_match_one_domain", "admin.example.com", "tcp", "HostSNI(`admin.example.com`)"},
		{"must_match_many_domains", "admin.example.com, other.example.com", "tcp", "HostSNI(`admin.example.com`) || HostSNI(`other.example.com`)"},
		{"must_keep_http_default", "", "http", "PathPrefix(`/`)"},
		{"must_match_http_domains", "a.example.com,b.example.com", "http", "Host(`a.example.com`) || Host(`b.example.com`)"},
	} {
		t.Run(tc.title, func(t *testing.T) {
			rule, err := hostRule(tc.domains, tc.protocol)
			require.NoError(t, err)
			require.Equal(t, tc.want, rule)
		})
	}
	for _, domain := range []string{"https://admin.example.com", "admin.example.com:443", "admin.example.com/path", "bad`host", "-bad.example.com", "a..example.com", "127.0.0.1", "*.example.com", "a.example.com,", " "} {
		t.Run("must_reject_"+domain, func(t *testing.T) { _, err := hostRule(domain, "tcp"); require.Error(t, err) })
	}
}

func TestDomainFlag(t *testing.T) {
	t.Run("must_accept_repeated_and_comma_separated_domains", func(t *testing.T) {
		var domains string
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.Var(domainFlag{&domains}, "domain", "domains")
		require.NoError(t, flags.Parse([]string{"--domain", "a.example.com", "--domain", "b.example.com,c.example.com"}))
		require.Equal(t, "a.example.com,b.example.com,c.example.com", domains)
	})
}

func TestValidatePassthrough(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	static := filepath.Join(t.TempDir(), "traefik.yaml")
	require.NoError(t, os.WriteFile(static, []byte("entryPoints:\n  custom:\n    address: ':8443'\n    http:\n      tls: {}\n  secure:\n    address: ':443'\n  websecure:\n    address: ':80'\n"), 0600))
	for _, tc := range []struct{ title, protocol, domains, entrypoint, file string }{
		{"must_allow_standard_tls_entrypoint", "tcp", "admin.example.com", "websecure", missing},
		{"must_allow_wildcard", "tcp", "*", "websecure", missing},
		{"must_allow_custom_tls_entrypoint", "tcp", "admin.example.com", "custom", static},
		{"must_allow_port_443", "tcp", "admin.example.com", "secure", static},
	} {
		t.Run(tc.title, func(t *testing.T) {
			require.NoError(t, validatePassthrough(tc.protocol, tc.domains, []string{tc.entrypoint}, true, tc.file))
		})
	}
	for _, tc := range []struct{ title, protocol, domains, entrypoint, file string }{
		{"must_reject_http", "http", "admin.example.com", "websecure", missing},
		{"must_require_explicit_domain", "tcp", "", "websecure", missing},
		{"must_reject_plain_entrypoint", "tcp", "admin.example.com", "web", missing},
		{"must_check_existing_static_config", "tcp", "admin.example.com", "websecure", static},
		{"must_reject_invalid_domain", "tcp", "https://example.com", "secure", static},
	} {
		t.Run(tc.title, func(t *testing.T) {
			require.Error(t, validatePassthrough(tc.protocol, tc.domains, []string{tc.entrypoint}, true, tc.file))
		})
	}
	require.NoError(t, validatePassthrough("http", "", nil, false, missing))
	require.Error(t, validatePassthrough("tcp", "*", nil, true, missing))
}

func TestValidateResourceName(t *testing.T) {
	for _, name := range []string{"vpn-only", "route_1", "route.example"} {
		require.NoError(t, validateResourceName(name))
	}
	for _, name := range []string{"", "../route", "route@file", "route/name", "route name"} {
		require.Error(t, validateResourceName(name))
	}
	require.Equal(t, []string{"one", "two", "three"}, flattenCSV([]string{"one,two", "three"}))
}
