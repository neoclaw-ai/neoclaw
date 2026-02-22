package approval

import "testing"

func TestEvaluateCommandPatterns_ExactMatch(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		allow    []string
		expected commandMatchDecision
	}{
		{
			name:     "exact match allowed",
			command:  "git status",
			allow:    []string{"git status"},
			expected: commandAllowed,
		},
		{
			name:     "exact pattern does not match extra args",
			command:  "git status --short",
			allow:    []string{"git status"},
			expected: commandNoMatch,
		},
		{
			name:     "single token exact",
			command:  "ls",
			allow:    []string{"ls"},
			expected: commandAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCommandPatterns(tt.command, tt.allow, nil)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestEvaluateCommandPatterns_Wildcards(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		pattern  string
		expected commandMatchDecision
	}{
		{
			name:     "trailing wildcard matches with args",
			command:  `git commit -m "fix"`,
			pattern:  "git commit *",
			expected: commandAllowed,
		},
		{
			name:     "trailing wildcard matches zero tokens",
			command:  "git commit",
			pattern:  "git commit *",
			expected: commandAllowed,
		},
		{
			name:     "word boundary token mismatch",
			command:  "git committed",
			pattern:  "git commit *",
			expected: commandNoMatch,
		},
		{
			name:     "ls wildcard",
			command:  "ls -la",
			pattern:  "ls *",
			expected: commandAllowed,
		},
		{
			name:     "lsof does not match ls wildcard",
			command:  "lsof",
			pattern:  "ls *",
			expected: commandNoMatch,
		},
		{
			name:     "mid wildcard single token",
			command:  "git checkout main",
			pattern:  "git * main",
			expected: commandAllowed,
		},
		{
			name:     "mid wildcard many tokens",
			command:  "git push origin main",
			pattern:  "git * main",
			expected: commandAllowed,
		},
		{
			name:     "mid wildcard zero tokens",
			command:  "git main",
			pattern:  "git * main",
			expected: commandAllowed,
		},
		{
			name:     "mid wildcard mismatch tail",
			command:  "git checkout feature",
			pattern:  "git * main",
			expected: commandNoMatch,
		},
		{
			name:     "leading wildcard match",
			command:  "git commit --help",
			pattern:  "* --help",
			expected: commandAllowed,
		},
		{
			name:     "leading wildcard no match",
			command:  "ls -la",
			pattern:  "* --help",
			expected: commandNoMatch,
		},
		{
			name:     "wildcard matches operators",
			command:  `git commit -m "x" && echo done`,
			pattern:  "git commit *",
			expected: commandAllowed,
		},
		{
			name:     "wildcard matches subshell token",
			command:  `git commit -m "$(cat msg.txt)"`,
			pattern:  "git commit *",
			expected: commandAllowed,
		},
		{
			name:     "quoted args preserved",
			command:  `grep -r "some pattern" /path`,
			pattern:  "grep *",
			expected: commandAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCommandPatterns(tt.command, []string{tt.pattern}, nil)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestEvaluateCommandPatterns_EnvPrefixAndPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		allow    []string
		deny     []string
		expected commandMatchDecision
	}{
		{
			name:     "env prefix stripped exact",
			command:  "FOO=bar git status",
			allow:    []string{"git status"},
			expected: commandAllowed,
		},
		{
			name:     "multiple env prefixes stripped",
			command:  `FOO=bar GIT_AUTHOR=x git commit -m "x"`,
			allow:    []string{"git commit *"},
			expected: commandAllowed,
		},
		{
			name:     "only env assignments no match",
			command:  "FOO=bar",
			allow:    []string{"git status"},
			expected: commandNoMatch,
		},
		{
			name:     "deny wins over allow",
			command:  "git push origin main",
			allow:    []string{"git *"},
			deny:     []string{"git push *"},
			expected: commandDenied,
		},
		{
			name:     "allow when deny does not match",
			command:  `git commit -m "x"`,
			allow:    []string{"git *"},
			deny:     []string{"git push *"},
			expected: commandAllowed,
		},
		{
			name:     "identical allow and deny still denied",
			command:  "curl https://example.com",
			allow:    []string{"curl *"},
			deny:     []string{"curl *"},
			expected: commandDenied,
		},
		{
			name:     "command parse error no match",
			command:  `git commit -m "unmatched quote`,
			allow:    []string{"git commit *"},
			expected: commandNoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCommandPatterns(tt.command, tt.allow, tt.deny)
			if got != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestGenerateCommandPattern(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		want       string
		wantExists bool
	}{
		{
			name:       "git commit with flag",
			command:    `git commit -m "fix bug"`,
			want:       "git commit *",
			wantExists: true,
		},
		{
			name:       "curl",
			command:    "curl -X POST https://example.com",
			want:       "curl *",
			wantExists: true,
		},
		{
			name:       "npm run build with flag",
			command:    "npm run build --watch",
			want:       "npm run build *",
			wantExists: true,
		},
		{
			name:       "python script with flag",
			command:    "python3 script.py --arg val",
			want:       "python3 script.py *",
			wantExists: true,
		},
		{
			name:       "ls with flag first arg",
			command:    "ls -la /tmp",
			want:       "ls *",
			wantExists: true,
		},
		{
			name:       "no flags git status",
			command:    "git status",
			want:       "git status",
			wantExists: true,
		},
		{
			name:       "no flags go build",
			command:    "go build ./...",
			want:       "go build ./...",
			wantExists: true,
		},
		{
			name:       "env stripped first",
			command:    `FOO=bar git commit -m "x"`,
			want:       "git commit *",
			wantExists: true,
		},
		{
			name:       "docker run",
			command:    "docker run --rm -it ubuntu bash",
			want:       "docker run *",
			wantExists: true,
		},
		{
			name:       "no command after env assignments",
			command:    "FOO=bar BAR=baz",
			wantExists: false,
		},
		{
			name:       "starts with flag no pattern",
			command:    "--version",
			wantExists: false,
		},
		{
			name:       "parse error no pattern",
			command:    `git commit -m "unmatched`,
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := generateCommandPattern(tt.command)
			if ok != tt.wantExists {
				t.Fatalf("expected exists=%t, got %t (pattern=%q)", tt.wantExists, ok, got)
			}
			if ok && got != tt.want {
				t.Fatalf("expected pattern %q, got %q", tt.want, got)
			}
		})
	}
}
