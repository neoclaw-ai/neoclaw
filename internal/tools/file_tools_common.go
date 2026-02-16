package tools

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxReadFileBytes = 50 * 1024

func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("argument %q must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("argument %q cannot be empty", key)
	}
	return s, nil
}

func resolveInputPath(workspaceDir, input string) (string, error) {
	if filepath.IsAbs(input) {
		return filepath.Clean(input), nil
	}
	if strings.TrimSpace(workspaceDir) == "" {
		return "", errors.New("workspace directory is required for relative paths")
	}
	return filepath.Clean(filepath.Join(workspaceDir, input)), nil
}

func resolveWorkspacePath(workspaceDir, input string) (string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		return "", errors.New("workspace directory is required")
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}

	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(workspaceAbs, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q is outside workspace", input)
	}
	return candidate, nil
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	return !utf8.Valid(data)
}
