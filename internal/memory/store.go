package memory

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// LogEntry is one timestamped daily log entry.
type LogEntry struct {
	Timestamp time.Time
	Entry     string
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

	path := filepath.Join(dailyDir, dailyLogFilename(now))
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

	if _, err := f.WriteString(formatDailyLogLine(LogEntry{Timestamp: now, Entry: entry}) + "\n"); err != nil {
		return fmt.Errorf("append daily log: %w", err)
	}
	return nil
}

// GetDailyLogs returns parsed daily log entries in the inclusive [fromTime, toTime] range.
func (s *Store) GetDailyLogs(fromTime, toTime time.Time) ([]LogEntry, error) {
	fromBound, toBound, err := normalizeTimeRange(fromTime, toTime)
	if err != nil {
		return nil, err
	}
	dailyDir, err := s.dailyDirPath()
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dailyDir)
	if errors.Is(err, os.ErrNotExist) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read daily log directory %q: %w", dailyDir, err)
	}

	logFiles := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		logFiles = append(logFiles, f.Name())
	}
	sort.Strings(logFiles)

	results := make([]LogEntry, 0)
	for _, name := range logFiles {
		dayText := strings.TrimSuffix(name, ".md")
		day, err := time.ParseInLocation("2006-01-02", dayText, time.Local)
		if err != nil {
			continue
		}
		path := filepath.Join(dailyDir, name)
		lines, err := readAllLines(path)
		if err != nil {
			return nil, fmt.Errorf("read daily log %q: %w", path, err)
		}
		for _, line := range lines {
			entry, ok := parseDailyLogLine(day, line)
			if !ok {
				continue
			}
			if entry.Timestamp.Before(fromBound) || entry.Timestamp.After(toBound) {
				continue
			}
			results = append(results, entry)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})
	return results, nil
}

// GetAllDailyLogs returns all parsed daily log entries.
func (s *Store) GetAllDailyLogs() ([]LogEntry, error) {
	return s.GetDailyLogs(time.Time{}, time.Time{})
}

// SearchLogs does case-insensitive substring search across daily logs in the time range.
func (s *Store) SearchLogs(query string, fromTime, toTime time.Time) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("query is required")
	}
	entries, err := s.GetDailyLogs(fromTime, toTime)
	if err != nil {
		return "", err
	}

	lowerQuery := strings.ToLower(query)
	var out strings.Builder
	for _, entry := range entries {
		line := formatDailyLogLine(entry)
		if !strings.Contains(strings.ToLower(line), lowerQuery) {
			continue
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(entry.Timestamp.Format(time.RFC3339))
		out.WriteString(" ")
		out.WriteString(line)
	}

	if out.Len() == 0 {
		return "no matches", nil
	}
	return out.String(), nil
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

func dailyLogFilename(ts time.Time) string {
	return ts.Format("2006-01-02") + ".md"
}

func formatDailyLogLine(entry LogEntry) string {
	return fmt.Sprintf("- %s: %s", entry.Timestamp.Format("15:04:05"), entry.Entry)
}

func parseDailyLogLine(day time.Time, line string) (LogEntry, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- ") {
		return LogEntry{}, false
	}
	rest := strings.TrimPrefix(line, "- ")
	sep := strings.Index(rest, ": ")
	if sep <= 0 {
		return LogEntry{}, false
	}
	timePart := strings.TrimSpace(rest[:sep])
	entry := rest[sep+2:]
	if strings.TrimSpace(entry) == "" {
		return LogEntry{}, false
	}
	ts, err := time.ParseInLocation(
		"2006-01-02 15:04:05",
		day.Format("2006-01-02")+" "+timePart,
		time.Local,
	)
	if err != nil {
		return LogEntry{}, false
	}
	return LogEntry{Timestamp: ts, Entry: entry}, true
}

func normalizeTimeRange(fromTime, toTime time.Time) (time.Time, time.Time, error) {
	fromBound := fromTime
	if fromBound.IsZero() {
		fromBound = time.Time{}
	}
	toBound := toTime
	if toBound.IsZero() {
		toBound = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	}
	if fromBound.After(toBound) {
		return time.Time{}, time.Time{}, errors.New("fromTime must be before or equal to toTime")
	}
	return fromBound, toBound, nil
}

func readAllLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lines := make([]string, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %q: %w", path, err)
	}
	return lines, nil
}
