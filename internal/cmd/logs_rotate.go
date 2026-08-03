package cmd

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/eliasmeireles/traefikctl/internal/logger"
)

var (
	rotateMaxSize  string
	rotateKeep     int
	rotateCompress bool
	rotateForce    bool
	rotateFiles    []string
)

var logsRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Truncate Traefik log files when they exceed the configured size",
	Long: `Rotate Traefik log files in-place.

When a target file exceeds --max-size, it is truncated to zero bytes
without releasing the inode, so Traefik keeps writing to the same
file descriptor — no SIGUSR1 or restart needed.

When --keep > 0, the current contents are copied to file.1 (file.2,
file.3, ...) before truncation, so up to --keep historical copies
are preserved.

Examples:
  sudo traefikctl logs rotate                        # truncate at 100MB (default)
  sudo traefikctl logs rotate --max-size 50M
  sudo traefikctl logs rotate --max-size 200M --keep 3 --compress
  sudo traefikctl logs rotate --force                # rotate regardless of size`,
	SilenceUsage: true,
	RunE:         runLogsRotate,
}

func init() {
	logsRotateCmd.Flags().StringVar(&rotateMaxSize, "max-size", "100M", "Max size before rotation (e.g. 50M, 1G)")
	logsRotateCmd.Flags().IntVar(&rotateKeep, "keep", 0, "Number of rotated copies to keep (0 = just truncate)")
	logsRotateCmd.Flags().BoolVar(&rotateCompress, "compress", false, "Gzip rotated copies (used with --keep > 0)")
	logsRotateCmd.Flags().BoolVar(&rotateForce, "force", false, "Rotate even if below threshold")
	logsRotateCmd.Flags().StringSliceVar(&rotateFiles, "file", nil, "Override target files (default: traefik.log + access.log)")
	logsCmd.AddCommand(logsRotateCmd)
}

func runLogsRotate(cmd *cobra.Command, args []string) error {
	if err := requireRoot(); err != nil {
		return err
	}

	maxBytes, err := parseSize(rotateMaxSize)
	if err != nil {
		return fmt.Errorf("invalid --max-size %q: %w", rotateMaxSize, err)
	}
	if rotateKeep < 0 {
		return fmt.Errorf("--keep must be >= 0")
	}

	files := rotateFiles
	if len(files) == 0 {
		files = []string{traefikLogPath, traefikAccessPath}
	}

	rotated := 0
	for _, path := range files {
		ok, err := rotateOne(path, maxBytes, rotateKeep, rotateCompress, rotateForce)
		if err != nil {
			logger.Error("rotate %s: %v", path, err)
			continue
		}
		if ok {
			rotated++
		}
	}

	if rotated == 0 && !rotateForce {
		logger.Info("No files needed rotation (all below %s)", formatBytes(maxBytes))
	}
	return nil
}

// rotateOne rotates a single file. Returns true if a rotation actually happened.
//
// Truncate-in-place is intentional: Traefik holds an open file descriptor on the
// inode, so renaming or unlinking would orphan its writes. truncate(2) on the
// existing path keeps the inode and the fd valid, while resetting the size to 0
// from the kernel's perspective — newly written bytes start at offset 0 again.
func rotateOne(path string, maxBytes int64, keep int, compress, force bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory", path)
	}
	if !force && info.Size() < maxBytes {
		return false, nil
	}

	logger.Info("Rotating %s (size: %s, threshold: %s)",
		path, formatBytes(info.Size()), formatBytes(maxBytes))

	if keep > 0 {
		if err := shiftBackups(path, keep, compress); err != nil {
			return false, fmt.Errorf("shift backups: %w", err)
		}
	}

	if err := os.Truncate(path, 0); err != nil {
		return false, fmt.Errorf("truncate: %w", err)
	}
	return true, nil
}

// shiftBackups rotates path.1 → path.2 → … → path.{keep}, dropping anything
// beyond `keep`, then writes the current path contents to path.1 (optionally
// gzipped). Orphans from a previous larger `keep` value are also removed so the
// final state has at most `keep` numbered backups.
func shiftBackups(path string, keep int, compress bool) error {
	suffix := ""
	if compress {
		suffix = ".gz"
	}

	for i := keep; ; i++ {
		p := backupName(path, i, suffix)
		if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
			break
		}
		_ = os.Remove(p)
	}

	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(backupName(path, i, suffix), backupName(path, i+1, suffix))
	}

	dst := backupName(path, 1, suffix)
	if compress {
		return gzipCopy(path, dst)
	}
	return copyFile(path, dst)
}

func backupName(path string, idx int, suffix string) string {
	return fmt.Sprintf("%s.%d%s", path, idx, suffix)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func gzipCopy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	gz := gzip.NewWriter(out)
	if _, err := io.Copy(gz, in); err != nil {
		_ = gz.Close()
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	return out.Sync()
}

// parseSize accepts forms like "100", "100B", "100K", "100M", "1G" (case-insensitive).
// A bare number is interpreted as bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}

	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "G"):
		mult = 1 << 30
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(s, "M"):
		mult = 1 << 20
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "K"):
		mult = 1 << 10
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	s = strings.TrimSpace(s)

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number")
	}
	if n < 0 {
		return 0, fmt.Errorf("must be >= 0")
	}
	return n * mult, nil
}

func formatBytes(b int64) string {
	const k = 1024
	switch {
	case b < k:
		return fmt.Sprintf("%dB", b)
	case b < k*k:
		return fmt.Sprintf("%.1fK", float64(b)/k)
	case b < k*k*k:
		return fmt.Sprintf("%.1fM", float64(b)/(k*k))
	default:
		return fmt.Sprintf("%.1fG", float64(b)/(k*k*k))
	}
}
