package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultMaxFileSize guards against opening pathologically large files.
const defaultMaxFileSize = 64 << 20 // 64 MiB

// withinWorkspace reports an error when target is not inside base (or equal
// to it), based on the cleaned relative path between the two.
func withinWorkspace(base, target string) error {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return fmt.Errorf("compute relative path failed: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path escapes workspace")
	}
	return nil
}

// resolveWorkspaceTarget validates that input is a non-empty relative path
// lexically contained in workDir and returns the cleaned absolute target plus
// the symlink-resolved workDir. Symlink containment of the target (read) or
// its parent chain (write) is left to the caller.
func resolveWorkspaceTarget(workDir, input string) (target, resolvedWorkDir string, err error) {
	if input == "" {
		return "", "", errors.New("path cannot be empty")
	}
	if filepath.IsAbs(input) {
		return "", "", fmt.Errorf("path must be relative to workspace, got absolute path: %s", input)
	}

	workDirAbs, err := filepath.Abs(workDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace directory failed: %w", err)
	}
	workDirAbs = filepath.Clean(workDirAbs)

	target = filepath.Join(workDirAbs, filepath.Clean(input))

	// Lexical containment: the cleaned joined path must stay inside workDirAbs.
	if err := withinWorkspace(workDirAbs, target); err != nil {
		return "", "", fmt.Errorf("path %q escapes workspace: %w", input, err)
	}

	// Resolve workDir too, since it may itself live behind symlinks (e.g. /tmp
	// on macOS points to /private/tmp), so both sides compare resolved paths.
	resolvedWorkDir, err = filepath.EvalSymlinks(workDirAbs)
	if err != nil {
		resolvedWorkDir = workDirAbs
	}
	return target, resolvedWorkDir, nil
}

// openValidatedFile opens path and returns the handle plus its size after
// validating it is a regular file not exceeding maxSize. Callers own the
// close and should register a context.AfterFunc so a cancelled context
// unblocks any pending read.
func openValidatedFile(path, displayPath string, maxSize int64) (*os.File, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open file failed, fail info: %s", err.Error())
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("stat file failed, fail info: %s", err.Error())
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, 0, fmt.Errorf("path %q is a directory, not a file", displayPath)
	}
	if info.Size() > maxSize {
		_ = f.Close()
		return nil, 0, fmt.Errorf("file size %d bytes exceeds limit %d bytes", info.Size(), maxSize)
	}
	return f, info.Size(), nil
}

// writeFileAtomic writes content to path via a temp file in the same
// directory followed by a rename, so readers never observe a partially
// written file. The rename is the commit point: cancellation is honored up
// to it, and once it starts the write completes.
func writeFileAtomic(ctx context.Context, path string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tiny-claw-write-*")
	if err != nil {
		return fmt.Errorf("create temp file failed, fail info: %s", err.Error())
	}
	tmpName := tmp.Name()
	// Clean up the temp file on any failure path; after a successful rename
	// the name no longer exists, so the removal is a no-op.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file failed, fail info: %s", err.Error())
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file failed, fail info: %s", err.Error())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file failed, fail info: %s", err.Error())
	}
	// CreateTemp creates files with 0o600; regular source files should be
	// group/other readable like os.WriteFile would produce.
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp file failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file failed, fail info: %s", err.Error())
	}
	return nil
}
