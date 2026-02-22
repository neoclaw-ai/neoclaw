package approval

import (
	"strings"

	"github.com/google/shlex"
)

type commandMatchDecision int

const (
	commandNoMatch commandMatchDecision = iota
	commandAllowed
	commandDenied
)

// Apply deny-first, then allow, over tokenized command patterns.
func evaluateCommandPatterns(command string, allowPatterns, denyPatterns []string) commandMatchDecision {
	commandTokens, err := tokenizeCommand(command)
	if err != nil || len(commandTokens) == 0 {
		return commandNoMatch
	}

	if matchCommandPatterns(denyPatterns, commandTokens) {
		return commandDenied
	}
	if matchCommandPatterns(allowPatterns, commandTokens) {
		return commandAllowed
	}
	return commandNoMatch
}

// Check whether any pattern matches the tokenized command.
func matchCommandPatterns(patterns []string, commandTokens []string) bool {
	for _, pattern := range patterns {
		patternTokens, err := tokenizeCommand(pattern)
		if err != nil {
			continue
		}
		if matchPatternTokens(patternTokens, commandTokens) {
			return true
		}
	}
	return false
}

// Match pattern tokens where "*" matches zero or more whole command tokens.
func matchPatternTokens(patternTokens, commandTokens []string) bool {
	if len(patternTokens) == 0 {
		return len(commandTokens) == 0
	}

	if patternTokens[0] == "*" {
		for i := 0; i <= len(commandTokens); i++ {
			if matchPatternTokens(patternTokens[1:], commandTokens[i:]) {
				return true
			}
		}
		return false
	}

	if len(commandTokens) == 0 || patternTokens[0] != commandTokens[0] {
		return false
	}

	return matchPatternTokens(patternTokens[1:], commandTokens[1:])
}

// Derive a persistent approval pattern from a raw command string.
func generateCommandPattern(command string) (string, bool) {
	tokens, err := tokenizeCommand(command)
	if err != nil || len(tokens) == 0 {
		return "", false
	}

	collected := make([]string, 0, len(tokens))
	foundFlag := false
	for _, token := range tokens {
		if strings.HasPrefix(token, "--") || strings.HasPrefix(token, "-") {
			foundFlag = true
			break
		}
		collected = append(collected, token)
	}

	if len(collected) == 0 {
		return "", false
	}

	pattern := strings.Join(collected, " ")
	if foundFlag {
		return pattern + " *", true
	}
	return pattern, true
}

// Parse shell tokens and strip leading KEY=value env assignments.
func tokenizeCommand(raw string) ([]string, error) {
	tokens, err := shlex.Split(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return stripLeadingEnvAssignments(tokens), nil
}

// Remove env prefix tokens so policy matching starts at the command.
func stripLeadingEnvAssignments(tokens []string) []string {
	index := 0
	for index < len(tokens) && isEnvAssignmentToken(tokens[index]) {
		index++
	}
	return tokens[index:]
}

// Report whether token is a shell env assignment like KEY=value.
func isEnvAssignmentToken(token string) bool {
	equalIndex := strings.IndexRune(token, '=')
	if equalIndex <= 0 {
		return false
	}

	key := token[:equalIndex]
	for i, ch := range key {
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_') {
				return false
			}
			continue
		}
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}
