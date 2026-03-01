package memory

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/neoclaw-ai/neoclaw/internal/logging"
)

var kvKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]+$`)

// LLMFormatter formats an entry for LLM context injection.
type LLMFormatter interface {
	FormatLLM() string
}

// LogEntry is one parsed TSV row from memory.tsv or a daily log file.
type LogEntry struct {
	Timestamp time.Time
	Tags      []string
	Text      string
	KV        string
}

// MarshalTSV returns the entry as a []string row for use with encoding/csv Writer.
func (e LogEntry) MarshalTSV() []string {
	tags := strings.Join(NormalizeTags(e.Tags), ",")
	text := sanitizeTSVField(e.Text)
	kv := sanitizeTSVField(e.KV)
	if kv == "" {
		kv = "-"
	}
	return []string{
		e.Timestamp.Format(time.RFC3339Nano),
		tags,
		text,
		kv,
	}
}

// UnmarshalTSV populates the entry from a []string row from encoding/csv Reader.
func (e *LogEntry) UnmarshalTSV(fields []string) error {
	if len(fields) != 4 {
		return fmt.Errorf("expected 4 fields, got %d", len(fields))
	}
	ts, err := time.Parse(time.RFC3339Nano, fields[0])
	if err != nil {
		return err
	}
	tags := []string{}
	for _, tag := range strings.Split(fields[1], ",") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}

	e.Timestamp = ts
	e.Tags = tags
	e.Text = fields[2]
	e.KV = fields[3]
	return nil
}

// FormatLLM formats the tags, text, and kv columns as a tab-separated string.
func (e LogEntry) FormatLLM() string {
	return strings.Join([]string{
		strings.Join(e.Tags, ","),
		e.Text,
		e.KV,
	}, "\t")
}

// NormalizeTags returns tags lowercased, with spaces replaced by underscores,
// duplicates removed, and first-tag position preserved.
func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		tag = strings.ReplaceAll(tag, " ", "_")
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

// ParseKV parses a raw KV string into a map on demand.
func ParseKV(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return map[string]string{}
	}

	values := make(map[string]string)
	for _, token := range strings.Split(raw, " ") {
		if token == "" {
			continue
		}
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			logging.Logger().Debug("skip malformed kv token", "token", token, "reason", "missing_separator")
			continue
		}
		key := parts[0]
		if !kvKeyPattern.MatchString(key) {
			logging.Logger().Debug("skip malformed kv token", "token", token, "reason", "invalid_key")
			continue
		}
		value := parts[1]
		if strings.ContainsAny(value, "\t\r\n") {
			logging.Logger().Debug("skip malformed kv token", "token", token, "reason", "invalid_value")
			continue
		}
		values[key] = value
	}
	return values
}

// sanitizeTSVField strips tabs and newlines so fields stay single-line and unquoted.
func sanitizeTSVField(value string) string {
	replacer := strings.NewReplacer("\t", "", "\n", "", "\r", "")
	return strings.TrimSpace(replacer.Replace(value))
}
