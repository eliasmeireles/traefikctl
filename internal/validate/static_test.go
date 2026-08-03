package validate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRunner replays scripted healthcheck responses, one per call, and
// records the scratch config content it was given each time so tests can
// assert exactly what the strip loop removed.
type fakeRunner struct {
	responses []string // stderr to return on each successive Healthcheck call
	help      string
	seen      []string // config content passed to Healthcheck, per call
	call      int
}

func (f *fakeRunner) Healthcheck(_ context.Context, configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	f.seen = append(f.seen, string(data))

	if f.call >= len(f.responses) {
		return "", nil // parsed
	}
	resp := f.responses[f.call]
	f.call++
	return resp, nil
}

func (f *fakeRunner) Help(context.Context) (string, error) { return f.help, nil }
func (f *fakeRunner) Available() bool                      { return true }

const accessLogHelp = `
   --log.level (Default: "ERROR")
   --log.compress (Default: "false")
   --serverstransport.forwardingtimeouts.dialtimeout (Default: "0")
`

func unknownField(node string) string {
	return `{"level":"error","error":"command healthcheck error: field not found, node: ` + node + `","message":"Command error"}`
}

func TestStaticBytes(t *testing.T) {
	t.Run("given four rejected accessLog fields one at a time then reports all four in one call with original line numbers", func(t *testing.T) {
		content := []byte(`log:
  level: INFO
accessLog:
  format: common
  maxSize: 50
  maxAge: 7
  maxBackups: 3
  compress: true
`)
		runner := &fakeRunner{
			responses: []string{
				unknownField("maxSize"),
				unknownField("maxAge"),
				unknownField("maxBackups"),
				unknownField("compress"),
			},
			help: accessLogHelp,
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)

		require.Len(t, result.Findings, 4)
		var keys []string
		for _, f := range result.Findings {
			keys = append(keys, f.Key)
			assert.True(t, f.Certain)
			assert.Greater(t, f.Line, 0)
		}
		assert.ElementsMatch(t, []string{"maxSize", "maxAge", "maxBackups", "compress"}, keys)
		assert.Equal(t, 5, result.Iterations) // 4 strips + 1 final clean pass
		assert.False(t, result.Truncated)
	})

	t.Run("given compress rejected under accessLog while log.compress is schema-known then only the accessLog occurrence is reported", func(t *testing.T) {
		content := []byte(`log:
  compress: true
accessLog:
  compress: true
`)
		runner := &fakeRunner{
			responses: []string{unknownField("compress")},
			help:      accessLogHelp,
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)

		require.Len(t, result.Findings, 1)
		assert.Equal(t, "accesslog.compress", result.Findings[0].Path)
		assert.True(t, result.Findings[0].Certain)

		// log.compress must survive in the scratch content used on the
		// final (clean) pass.
		require.Len(t, runner.seen, 2)
		assert.Contains(t, runner.seen[1], "compress: true")
		assert.Contains(t, runner.seen[1], "log:")
	})

	t.Run("given schema knows neither occurrence then removes both and marks them uncertain", func(t *testing.T) {
		content := []byte(`log:
  weird: true
accessLog:
  weird: true
`)
		runner := &fakeRunner{
			responses: []string{unknownField("weird")},
			help:      accessLogHelp, // "weird" appears nowhere in this schema
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)

		require.Len(t, result.Findings, 2)
		assert.False(t, result.Findings[0].Certain)
		assert.False(t, result.Findings[1].Certain)
	})

	t.Run("given a rejected field that cannot be located as a literal key then stops immediately without truncation", func(t *testing.T) {
		content := []byte("log:\n  a: 1\n")
		runner := &fakeRunner{
			help:      accessLogHelp,
			responses: []string{unknownField("nonexistentkey")},
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner, MaxIterations: 3})
		require.NoError(t, err)
		require.Len(t, result.Findings, 1)
		assert.Contains(t, result.Findings[0].Message, "could not be located")
		assert.False(t, result.Truncated)
	})

	t.Run("given more distinct rejected fields than MaxIterations then stops early and reports truncated", func(t *testing.T) {
		content := []byte(`log:
  bad1: true
  bad2: true
  bad3: true
  bad4: true
  bad5: true
`)
		runner := &fakeRunner{
			help: accessLogHelp,
			responses: []string{
				unknownField("bad1"),
				unknownField("bad2"),
				unknownField("bad3"),
				unknownField("bad4"),
				unknownField("bad5"),
			},
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner, MaxIterations: 3})
		require.NoError(t, err)

		assert.True(t, result.Truncated)
		assert.Equal(t, 3, result.Iterations)
		assert.Len(t, result.Findings, 3)
	})

	t.Run("given a healthy config then reports OK with exactly one runner call", func(t *testing.T) {
		content := []byte("log:\n  level: INFO\n")
		runner := &fakeRunner{help: accessLogHelp}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)

		assert.True(t, result.OK())
		assert.Equal(t, 1, result.Iterations)
		assert.Len(t, runner.seen, 1)
	})

	t.Run("given the runner reports config parsed via the ping message then treats it as OK", func(t *testing.T) {
		content := []byte("log:\n  level: INFO\n")
		runner := &fakeRunner{
			responses: []string{"Error calling healthcheck: please enable `ping` to use health check\n"},
			help:      accessLogHelp,
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)
		assert.True(t, result.OK())
	})

	t.Run("given a runner that is unavailable then skips exec validation but still checks YAML syntax", func(t *testing.T) {
		content := []byte("log:\n  level: INFO\n")
		runner := &fakeRunner{}
		unavailable := &unavailableRunner{}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: unavailable})
		require.NoError(t, err)
		assert.True(t, result.Skipped)
		assert.NotEmpty(t, result.SkipReason)
		_ = runner // unused in this branch, kept for symmetry
	})

	t.Run("given invalid YAML syntax then reports a syntax finding without invoking the runner", func(t *testing.T) {
		content := []byte("log:\n  level: [unterminated\n")
		runner := &fakeRunner{help: accessLogHelp}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)
		require.Len(t, result.Findings, 1)
		assert.Contains(t, result.Findings[0].Message, "YAML syntax error")
		assert.Empty(t, runner.seen)
	})

	t.Run("given a rejected field whose section then reports empty standalone then removes the section silently without an extra finding", func(t *testing.T) {
		content := []byte(`log:
  level: INFO
serversTransport:
  dialTimeout: 5s
`)
		runner := &fakeRunner{
			responses: []string{
				unknownField("dialTimeout"),
				`{"level":"error","error":"command healthcheck error: serversTransport cannot be a standalone element (type *static.ServersTransport)","message":"Command error"}`,
			},
			help: accessLogHelp,
		}

		result, err := StaticBytes(context.Background(), "traefik.yaml", content, Options{Runner: runner})
		require.NoError(t, err)

		require.Len(t, result.Findings, 1)
		assert.Equal(t, "dialTimeout", result.Findings[0].Key)
		require.Len(t, runner.seen, 3)
		assert.NotContains(t, runner.seen[2], "serversTransport")
	})

	t.Run("given any validation outcome then the original content bytes are never touched", func(t *testing.T) {
		original := []byte(`log:
  compress: true
accessLog:
  compress: true
`)
		snapshot := append([]byte(nil), original...)
		runner := &fakeRunner{
			responses: []string{unknownField("compress")},
			help:      accessLogHelp,
		}

		_, err := StaticBytes(context.Background(), "traefik.yaml", original, Options{Runner: runner})
		require.NoError(t, err)
		assert.Equal(t, snapshot, original)
	})
}

type unavailableRunner struct{}

func (unavailableRunner) Healthcheck(context.Context, string) (string, error) { return "", nil }
func (unavailableRunner) Help(context.Context) (string, error)                { return "", nil }
func (unavailableRunner) Available() bool                                     { return false }
