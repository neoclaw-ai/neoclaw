// Package memory manages long-term memory and daily log files for an agent.
package memory

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/neoclaw-ai/neoclaw/internal/config"
	"github.com/neoclaw-ai/neoclaw/internal/logging"
	"github.com/neoclaw-ai/neoclaw/internal/store"
)

const maxLoggedChars = 200

// Store manages long-term memory and daily log files.
type Store struct {
	dir         string
	mu          sync.RWMutex
	dailyLog    []LogEntry
	memoryFacts []LogEntry
}

// New creates a Store for the given memory directory, loading existing TSV files into memory.
func New(dir string) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("memory directory is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("memory path %s is not a directory", dir)
	}

	s := &Store{dir: dir}
	dailyLog, err := s.loadDailyLog()
	if err != nil {
		return nil, err
	}
	memoryFacts, err := s.loadMemoryFacts()
	if err != nil {
		return nil, err
	}
	s.dailyLog = dailyLog
	s.memoryFacts = memoryFacts
	return s, nil
}

// AppendDailyLog appends an entry to today's daily log.
func (s *Store) AppendDailyLog(entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if strings.TrimSpace(entry.Text) == "" {
		return errors.New("entry text is required")
	}
	entry = normalizeEntryForWrite(entry)

	dailyDir, err := s.dailyDirPath()
	if err != nil {
		return err
	}
	path := filepath.Join(dailyDir, entry.Timestamp.Format("2006-01-02")+".tsv")
	if err := appendTSVRow(path, entry.MarshalTSV()); err != nil {
		return err
	}
	s.dailyLog = append(s.dailyLog, entry)
	sortEntries(s.dailyLog)
	logging.Logger().Debug(
		"memory write",
		"operation", "append_daily_log",
		"file", filepath.Base(path),
		"entry", truncateForLog(entry.Text, maxLoggedChars),
	)
	return nil
}

// AppendMemory appends a persistent fact to memory.tsv.
func (s *Store) AppendMemory(entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if strings.TrimSpace(entry.Text) == "" {
		return errors.New("entry text is required")
	}
	entry = normalizeEntryForWrite(entry)

	path := filepath.Join(s.dir, config.MemoryFilePath)
	if err := appendTSVRow(path, entry.MarshalTSV()); err != nil {
		return err
	}
	s.memoryFacts = append(s.memoryFacts, entry)
	sortEntries(s.memoryFacts)
	logging.Logger().Debug(
		"memory write",
		"operation", "append_memory",
		"file", filepath.Base(path),
		"entry", truncateForLog(entry.Text, maxLoggedChars),
	)
	return nil
}

// Search searches both daily log and memory fact entries using a regex pattern.
func (s *Store) Search(query string, fromTime, toTime time.Time) ([]LogEntry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query is required")
	}
	pattern, err := regexp.Compile(query)
	if err != nil {
		return nil, err
	}

	fromBound := fromTime
	toBound := toTime
	if toBound.IsZero() || toBound.Before(fromBound) {
		toBound = farFutureTime()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]LogEntry, 0)
	for _, entry := range s.dailyLog {
		if entry.Timestamp.Before(fromBound) || entry.Timestamp.After(toBound) {
			continue
		}
		if !pattern.MatchString(formatTSVLine(entry)) {
			continue
		}
		results = append(results, entry)
	}
	for _, entry := range s.memoryFacts {
		if entry.Timestamp.Before(fromBound) || entry.Timestamp.After(toBound) {
			continue
		}
		if !pattern.MatchString(formatTSVLine(entry)) {
			continue
		}
		results = append(results, entry)
	}
	sortEntries(results)
	return results, nil
}

// ActiveFacts returns the deduplicated, expiry-filtered list of active persistent facts.
func (s *Store) ActiveFacts(now time.Time) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byTopic := make(map[string][]LogEntry)
	for _, entry := range s.memoryFacts {
		if len(entry.Tags) == 0 {
			continue
		}
		topic := entry.Tags[0]
		byTopic[topic] = append(byTopic[topic], entry)
	}

	active := make([]LogEntry, 0, len(byTopic))
	for _, entries := range byTopic {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.After(entries[j].Timestamp)
		})
		for _, entry := range entries {
			if isExpired(entry, now) {
				continue
			}
			active = append(active, entry)
			break
		}
	}
	sortEntries(active)
	return active
}

// DailyLogsByDate returns daily log entries whose local calendar date matches the provided dates.
func (s *Store) DailyLogsByDate(dates []time.Time) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed := make(map[string]struct{}, len(dates))
	for _, date := range dates {
		if date.IsZero() {
			continue
		}
		allowed[date.In(time.Local).Format("2006-01-02")] = struct{}{}
	}
	if len(allowed) == 0 {
		return []LogEntry{}
	}

	results := make([]LogEntry, 0)
	for _, entry := range s.dailyLog {
		day := entry.Timestamp.In(time.Local).Format("2006-01-02")
		if _, ok := allowed[day]; !ok {
			continue
		}
		results = append(results, entry)
	}
	return results
}

// FactTags returns first-tag counts across all memory facts, including superseded entries.
func (s *Store) FactTags() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for _, entry := range s.memoryFacts {
		if len(entry.Tags) == 0 {
			continue
		}
		counts[entry.Tags[0]]++
	}
	return counts
}

// GetDailyLogs returns parsed daily log entries in the inclusive [fromTime, toTime] range.
func (s *Store) GetDailyLogs(fromTime, toTime time.Time) ([]LogEntry, error) {
	fromBound := fromTime
	toBound := toTime
	if toBound.IsZero() {
		toBound = farFutureTime()
	}
	if fromBound.After(toBound) {
		return nil, errors.New("fromTime must be before or equal to toTime")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]LogEntry, 0, len(s.dailyLog))
	for _, entry := range s.dailyLog {
		if entry.Timestamp.Before(fromBound) || entry.Timestamp.After(toBound) {
			continue
		}
		results = append(results, entry)
	}
	return results, nil
}

func (s *Store) dailyDirPath() (string, error) {
	if strings.TrimSpace(s.dir) == "" {
		return "", errors.New("memory directory is required")
	}
	return filepath.Join(s.dir, config.DailyDirPath), nil
}

func (s *Store) loadDailyLog() ([]LogEntry, error) {
	dailyDir, err := s.dailyDirPath()
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dailyDir)
	if errors.Is(err, os.ErrNotExist) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read daily log directory %s: %w", dailyDir, err)
	}

	names := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".tsv") {
			continue
		}
		names = append(names, file.Name())
	}
	sort.Strings(names)

	entries := make([]LogEntry, 0)
	for _, name := range names {
		loaded, err := loadTSVFile(filepath.Join(dailyDir, name))
		if err != nil {
			return nil, err
		}
		entries = append(entries, loaded...)
	}
	sortEntries(entries)
	return entries, nil
}

func (s *Store) loadMemoryFacts() ([]LogEntry, error) {
	path := filepath.Join(s.dir, config.MemoryFilePath)
	entries, err := loadTSVFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	sortEntries(entries)
	return entries, nil
}

func loadTSVFile(path string) ([]LogEntry, error) {
	content, err := store.ReadFile(path)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1

	entries := make([]LogEntry, 0)
	for {
		fields, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			logging.Logger().Warn("skip malformed tsv row", "path", path, "err", err)
			continue
		}
		if len(fields) > 0 && fields[0] == "ts" {
			continue
		}
		var entry LogEntry
		if err := entry.UnmarshalTSV(fields); err != nil {
			logging.Logger().Warn("skip malformed tsv row", "path", path, "err", err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func normalizeEntryForWrite(entry LogEntry) LogEntry {
	entry.Tags = NormalizeTags(entry.Tags)
	if strings.TrimSpace(entry.KV) == "" {
		entry.KV = "-"
	}
	return entry
}

func appendTSVRow(path string, row []string) error {
	needsHeader := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		needsHeader = true
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if needsHeader {
		header, err := marshalTSVRows([]string{"ts", "tags", "text", "kv"})
		if err != nil {
			return err
		}
		if err := store.WriteFile(path, header); err != nil {
			return fmt.Errorf("initialize tsv file: %w", err)
		}
	}

	data, err := marshalTSVRows(row)
	if err != nil {
		return err
	}
	if err := store.AppendFile(path, data); err != nil {
		return fmt.Errorf("append tsv row: %w", err)
	}
	return nil
}

func marshalTSVRows(rows ...[]string) ([]byte, error) {
	var b strings.Builder
	writer := csv.NewWriter(&b)
	writer.Comma = '\t'
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write tsv row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush tsv row: %w", err)
	}
	return []byte(b.String()), nil
}

func sortEntries(entries []LogEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
}

func farFutureTime() time.Time {
	return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
}

func isExpired(entry LogEntry, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	expiresRaw := ParseKV(entry.KV)["expires"]
	if expiresRaw == "" {
		return false
	}
	expiresUnix, err := strconv.ParseInt(expiresRaw, 10, 64)
	if err != nil {
		return false
	}
	return !time.Unix(expiresUnix, 0).After(now)
}

func formatTSVLine(entry LogEntry) string {
	return strings.Join(entry.MarshalTSV(), "\t")
}

// truncateForLog bounds log field content so debug logs stay concise.
func truncateForLog(text string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	if utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	var b strings.Builder
	charCount := 0
	for _, r := range text {
		if charCount >= maxChars {
			break
		}
		b.WriteRune(r)
		charCount++
	}
	return fmt.Sprintf("%s...[truncated %d chars]", b.String(), utf8.RuneCountInString(text)-maxChars)
}
