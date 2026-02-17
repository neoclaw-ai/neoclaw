package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	memoryFileName = "memory.md"
	dailyDirName   = "daily"
)

// Store manages long-term memory and daily log files.
type Store struct {
	dir string
}

// New creates a Store for the given memory directory.
func New(dir string) *Store {
	return &Store{dir: dir}
}

// ReadMemory returns the contents of memory.md. Returns empty header if file doesn't exist.
func (s *Store) ReadMemory() (string, error) {
	path, err := s.memoryPath()
	if err != nil {
		return "", err
	}
	return readOrInitializeMemory(path)
}

// AppendFact adds a fact to a section in memory.md.
// No-op if the fact already exists in that section.
func (s *Store) AppendFact(section, fact string) error {
	section = strings.TrimSpace(section)
	fact = strings.TrimSpace(fact)
	if section == "" {
		return errors.New("section is required")
	}
	if fact == "" {
		return errors.New("fact is required")
	}

	path, err := s.memoryPath()
	if err != nil {
		return err
	}
	content, err := readOrInitializeMemory(path)
	if err != nil {
		return err
	}
	next, changed := addFact(content, section, fact)
	if !changed {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return fmt.Errorf("write memory file: %w", err)
	}
	return nil
}

// RemoveFact removes all matching bullet lines from memory.md. Returns count of removed lines.
func (s *Store) RemoveFact(fact string) (int, error) {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return 0, errors.New("fact is required")
	}

	path, err := s.memoryPath()
	if err != nil {
		return 0, err
	}
	content, err := readOrInitializeMemory(path)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	target := "- " + fact
	removed := 0
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == target {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}

	next := strings.Join(kept, "\n")
	if !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return 0, fmt.Errorf("write memory file: %w", err)
	}
	return removed, nil
}

// AppendDailyLog appends a timestamped entry to today's daily log file.
func (s *Store) AppendDailyLog(now time.Time, entry string) error {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return errors.New("entry is required")
	}
	dailyDir, err := s.dailyDirPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		return fmt.Errorf("create daily log directory: %w", err)
	}

	path := filepath.Join(dailyDir, now.Format("2006-01-02")+".md")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		header := "# " + now.Format("2006-01-02") + "\n\n"
		if err := os.WriteFile(path, []byte(header), 0o644); err != nil {
			return fmt.Errorf("initialize daily log: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat daily log: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open daily log: %w", err)
	}
	defer f.Close()

	line := fmt.Sprintf("- %s: %s\n", now.Format("15:04:05"), entry)
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("append daily log: %w", err)
	}
	return nil
}

// SearchLogs does case-insensitive substring search across daily logs for the last N days.
func (s *Store) SearchLogs(now time.Time, query string, daysBack int) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("query is required")
	}
	if daysBack <= 0 {
		daysBack = 7
	}

	dailyDir, err := s.dailyDirPath()
	if err != nil {
		return "", err
	}

	lowerQuery := strings.ToLower(query)
	var out strings.Builder
	matches := 0
	for i := 0; i < daysBack; i++ {
		day := now.AddDate(0, 0, -i)
		path := filepath.Join(dailyDir, day.Format("2006-01-02")+".md")
		raw, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read daily log %q: %w", path, err)
		}

		lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		dayMatches := make([]string, 0)
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				dayMatches = append(dayMatches, strings.TrimSpace(line))
			}
		}
		if len(dayMatches) == 0 {
			continue
		}

		if matches > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(day.Format("2006-01-02"))
		out.WriteByte('\n')
		for _, line := range dayMatches {
			out.WriteString("- ")
			out.WriteString(line)
			out.WriteByte('\n')
		}
		matches += len(dayMatches)
	}

	if matches == 0 {
		return "no matches", nil
	}
	return strings.TrimSpace(out.String()), nil
}

// LoadContext returns memory.md contents for system prompt injection.
func (s *Store) LoadContext() (memoryText string, err error) {
	memoryPath, err := s.memoryPath()
	if err != nil {
		return "", err
	}
	memoryText, err = readOptionalFile(memoryPath)
	if err != nil {
		return "", err
	}
	return memoryText, nil
}

// OptionalIntArg parses an optional integer argument from a tool args map.
func OptionalIntArg(args map[string]any, key string, def int) (int, error) {
	raw, ok := args[key]
	if !ok {
		return def, nil
	}
	switch v := raw.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, fmt.Errorf("argument %q must be an integer", key)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("argument %q must be an integer", key)
	}
}

func (s *Store) memoryPath() (string, error) {
	if strings.TrimSpace(s.dir) == "" {
		return "", errors.New("memory directory is required")
	}
	return filepath.Join(s.dir, memoryFileName), nil
}

func (s *Store) dailyDirPath() (string, error) {
	if strings.TrimSpace(s.dir) == "" {
		return "", errors.New("memory directory is required")
	}
	return filepath.Join(s.dir, dailyDirName), nil
}

func readOrInitializeMemory(path string) (string, error) {
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(raw) == 0 {
			return "# Memory\n", nil
		}
		content := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content, nil
	case errors.Is(err, os.ErrNotExist):
		return "# Memory\n", nil
	default:
		return "", fmt.Errorf("read memory file: %w", err)
	}
}

func addFact(content, section, fact string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# Memory" {
		lines = append([]string{"# Memory", ""}, lines...)
	}

	header := "## " + section
	sectionStart := -1
	for i := range lines {
		if strings.TrimSpace(lines[i]) == header {
			sectionStart = i
			break
		}
	}

	if sectionStart == -1 {
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, header, "- "+fact, "")
		return strings.Join(lines, "\n"), true
	}

	sectionEnd := len(lines)
	for i := sectionStart + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			sectionEnd = i
			break
		}
	}

	targetLine := "- " + fact
	for i := sectionStart + 1; i < sectionEnd; i++ {
		if strings.TrimSpace(lines[i]) == targetLine {
			return strings.Join(lines, "\n"), false
		}
	}

	insertAt := sectionEnd
	for insertAt > sectionStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}

	next := make([]string, 0, len(lines)+1)
	next = append(next, lines[:insertAt]...)
	next = append(next, targetLine)
	next = append(next, lines[insertAt:]...)
	return strings.Join(next, "\n"), true
}

func readOptionalFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		return string(raw), nil
	case errors.Is(err, os.ErrNotExist):
		return "", nil
	default:
		return "", fmt.Errorf("read %q: %w", path, err)
	}
}
