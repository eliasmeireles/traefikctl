package validate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	t.Run("given empty stderr and nil error then returns parsed", func(t *testing.T) {
		c := classify("", nil)
		assert.Equal(t, outcomeParsed, c.outcome)
	})

	t.Run("given empty stderr and a run error then returns runner error", func(t *testing.T) {
		c := classify("", errors.New("exec: not found"))
		assert.Equal(t, outcomeRunnerError, c.outcome)
		assert.Contains(t, c.raw, "not found")
	})

	t.Run("given healthcheck ping message then returns parsed", func(t *testing.T) {
		c := classify("Error calling healthcheck: please enable `ping` to use health check\n", nil)
		assert.Equal(t, outcomeParsed, c.outcome)
	})

	t.Run("given field not found JSON line then returns unknown field with node name", func(t *testing.T) {
		stderr := `{"level":"error","error":"command healthcheck error: field not found, node: compress","time":"2026-08-03T11:32:38-03:00","message":"Command error"}` + "\n"
		c := classify(stderr, nil)
		assert.Equal(t, outcomeUnknownField, c.outcome)
		assert.Equal(t, "compress", c.node)
	})

	t.Run("given field not found for dialTimeout then extracts dialTimeout", func(t *testing.T) {
		stderr := `{"level":"error","error":"command healthcheck error: field not found, node: dialTimeout","message":"Command error"}`
		c := classify(stderr, nil)
		assert.Equal(t, outcomeUnknownField, c.outcome)
		assert.Equal(t, "dialTimeout", c.node)
	})

	t.Run("given unsupported file extension message then returns bad extension", func(t *testing.T) {
		c := classify(`{"level":"error","error":"unsupported file extension: .txt","message":"Command error"}`, nil)
		assert.Equal(t, outcomeBadExtension, c.outcome)
	})

	t.Run("given no valid configuration message then returns no config", func(t *testing.T) {
		c := classify(`{"level":"error","error":"command healthcheck error: no valid configuration found in file: dynamic.yaml","message":"Command error"}`, nil)
		assert.Equal(t, outcomeNoConfig, c.outcome)
	})

	t.Run("given an unrecognized parse error then returns parse error with raw message", func(t *testing.T) {
		stderr := `{"level":"error","error":"command healthcheck error: something else entirely","message":"Command error"}`
		c := classify(stderr, nil)
		assert.Equal(t, outcomeParseError, c.outcome)
		assert.Contains(t, c.raw, "something else entirely")
	})

	t.Run("given plain non-JSON stderr with no known prefix then returns parse error", func(t *testing.T) {
		c := classify("some totally unstructured crash output\n", nil)
		assert.Equal(t, outcomeParseError, c.outcome)
	})

	t.Run("given a standalone-element complaint then returns empty section with its name", func(t *testing.T) {
		stderr := `{"level":"error","error":"command healthcheck error: serversTransport cannot be a standalone element (type *static.ServersTransport)","message":"Command error"}`
		c := classify(stderr, nil)
		assert.Equal(t, outcomeEmptySection, c.outcome)
		assert.Equal(t, "serversTransport", c.node)
	})
}
