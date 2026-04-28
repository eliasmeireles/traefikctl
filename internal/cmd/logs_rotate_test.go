package cmd

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	st, ok := info.Sys().(*syscall.Stat_t)
	require.True(t, ok, "expected *syscall.Stat_t on this platform")
	return st.Ino
}

func TestParseSize(t *testing.T) {
	t.Run("given bare integer then returns bytes", func(t *testing.T) {
		n, err := parseSize("4096")
		require.NoError(t, err)
		assert.Equal(t, int64(4096), n)
	})

	t.Run("given KB suffix then multiplies by 1024", func(t *testing.T) {
		n, err := parseSize("4K")
		require.NoError(t, err)
		assert.Equal(t, int64(4*1024), n)
	})

	t.Run("given MB suffix then multiplies by 1024^2", func(t *testing.T) {
		n, err := parseSize("100M")
		require.NoError(t, err)
		assert.Equal(t, int64(100*1024*1024), n)
	})

	t.Run("given GB suffix then multiplies by 1024^3", func(t *testing.T) {
		n, err := parseSize("2G")
		require.NoError(t, err)
		assert.Equal(t, int64(2*1024*1024*1024), n)
	})

	t.Run("given lowercase suffix then accepts it", func(t *testing.T) {
		n, err := parseSize("50m")
		require.NoError(t, err)
		assert.Equal(t, int64(50*1024*1024), n)
	})

	t.Run("given B suffix then treats as bytes", func(t *testing.T) {
		n, err := parseSize("512B")
		require.NoError(t, err)
		assert.Equal(t, int64(512), n)
	})

	t.Run("when input is empty then returns error", func(t *testing.T) {
		_, err := parseSize("")
		require.Error(t, err)
	})

	t.Run("when input is not a number then returns error", func(t *testing.T) {
		_, err := parseSize("abcM")
		require.Error(t, err)
	})

	t.Run("when input is negative then returns error", func(t *testing.T) {
		_, err := parseSize("-10M")
		require.Error(t, err)
	})
}

func TestFormatBytes(t *testing.T) {
	t.Run("must format bytes under 1K with B suffix", func(t *testing.T) {
		assert.Equal(t, "512B", formatBytes(512))
	})

	t.Run("must format kilobytes with K suffix", func(t *testing.T) {
		assert.Equal(t, "1.5K", formatBytes(1536))
	})

	t.Run("must format megabytes with M suffix", func(t *testing.T) {
		assert.Equal(t, "100.0M", formatBytes(100*1024*1024))
	})

	t.Run("must format gigabytes with G suffix", func(t *testing.T) {
		assert.Equal(t, "2.0G", formatBytes(2*1024*1024*1024))
	})
}

func TestRotateOne(t *testing.T) {
	t.Run("when file is below threshold then no rotation happens", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "small.log")
		require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

		ok, err := rotateOne(path, 1024, 0, false, false)
		require.NoError(t, err)
		assert.False(t, ok)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(data))
	})

	t.Run("when file exceeds threshold then file is truncated", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "big.log")
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("a", 2048)), 0o644))

		ok, err := rotateOne(path, 1024, 0, false, false)
		require.NoError(t, err)
		assert.True(t, ok)

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())
	})

	t.Run("given force flag then rotates regardless of size", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "small.log")
		require.NoError(t, os.WriteFile(path, []byte("hi"), 0o644))

		ok, err := rotateOne(path, 1<<30, 0, false, true)
		require.NoError(t, err)
		assert.True(t, ok)

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())
	})

	t.Run("when file does not exist then returns false without error", func(t *testing.T) {
		dir := t.TempDir()
		ok, err := rotateOne(filepath.Join(dir, "missing.log"), 1024, 0, false, false)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("when path is a directory then returns error", func(t *testing.T) {
		dir := t.TempDir()
		_, err := rotateOne(dir, 0, 0, false, true)
		require.Error(t, err)
	})

	t.Run("given keep > 0 then current contents are preserved as .1", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rolling.log")
		payload := strings.Repeat("x", 4096)
		require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

		ok, err := rotateOne(path, 1024, 3, false, false)
		require.NoError(t, err)
		assert.True(t, ok)

		live, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, int64(0), live.Size())

		backup, err := os.ReadFile(path + ".1")
		require.NoError(t, err)
		assert.Equal(t, payload, string(backup))
	})

	t.Run("given keep > 0 and compress then backup is gzipped", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rolling.log")
		payload := strings.Repeat("y", 4096)
		require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

		ok, err := rotateOne(path, 1024, 1, true, false)
		require.NoError(t, err)
		assert.True(t, ok)

		raw, err := os.Open(path + ".1.gz")
		require.NoError(t, err)
		defer func() { _ = raw.Close() }()

		gz, err := gzip.NewReader(raw)
		require.NoError(t, err)
		defer func() { _ = gz.Close() }()

		decoded, err := io.ReadAll(gz)
		require.NoError(t, err)
		assert.Equal(t, payload, string(decoded))
	})
}

func TestShiftBackups(t *testing.T) {
	t.Run("must shift older backups forward and drop those beyond keep", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rolling.log")
		require.NoError(t, os.WriteFile(path, []byte("current"), 0o644))
		require.NoError(t, os.WriteFile(path+".1", []byte("first"), 0o644))
		require.NoError(t, os.WriteFile(path+".2", []byte("second"), 0o644))

		require.NoError(t, shiftBackups(path, 2, false))

		got1, err := os.ReadFile(path + ".1")
		require.NoError(t, err)
		assert.Equal(t, "current", string(got1))

		got2, err := os.ReadFile(path + ".2")
		require.NoError(t, err)
		assert.Equal(t, "first", string(got2))

		_, err = os.Stat(path + ".3")
		assert.True(t, os.IsNotExist(err), "expected .3 to be removed when keep=2")
	})

	t.Run("must drop oldest beyond keep before shifting", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "rolling.log")
		require.NoError(t, os.WriteFile(path, []byte("current"), 0o644))
		require.NoError(t, os.WriteFile(path+".1", []byte("first"), 0o644))
		require.NoError(t, os.WriteFile(path+".2", []byte("second"), 0o644))
		require.NoError(t, os.WriteFile(path+".3", []byte("ancient"), 0o644))

		require.NoError(t, shiftBackups(path, 2, false))

		_, err := os.Stat(path + ".3")
		assert.True(t, os.IsNotExist(err), "expected .3 (beyond keep) to be removed")

		got1, err := os.ReadFile(path + ".1")
		require.NoError(t, err)
		assert.Equal(t, "current", string(got1))

		got2, err := os.ReadFile(path + ".2")
		require.NoError(t, err)
		assert.Equal(t, "first", string(got2))
	})
}

func TestRotateOnePreservesInode(t *testing.T) {
	t.Run("must keep the same inode after truncation so an open writer keeps writing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "open.log")

		writer, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		require.NoError(t, err)
		defer func() { _ = writer.Close() }()

		_, err = writer.Write([]byte(strings.Repeat("z", 4096)))
		require.NoError(t, err)

		inoBefore := inodeOf(t, path)

		ok, err := rotateOne(path, 1024, 0, false, false)
		require.NoError(t, err)
		require.True(t, ok)

		inoAfter := inodeOf(t, path)
		assert.Equal(t, inoBefore, inoAfter, "inode should be preserved across truncation")

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, int64(0), info.Size())

		_, err = writer.Write([]byte("post-rotate"))
		require.NoError(t, err)
	})
}
