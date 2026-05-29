package stats

import (
	"regexp"
	"strings"
	"testing"
)

func TestProcessCommand_BaseCmd(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "git status", "git"},
		{"env assign stripped", "TOKEN=secret git status", "git"},
		{"multiple env assigns", "A=1 B=2 go build ./...", "go"},
		{"redacted assign stripped", "TOKEN=REDACTED git status", "git"},
		{"sudo wrapper", "sudo git status", "git"},
		// consumeWrapper strips leading -flag tokens only; -u <arg> is not handled.
		{"sudo boolean flag", "sudo -n git status", "git"},
		{"env wrapper", "env GOPATH=/tmp go build", "go"},
		{"cd then command", "cd /foo && go test ./...", "go"},
		{"pipe first segment", "cat file.txt | grep foo", "cat"},
		{"empty after strip", "TOKEN=x", ""},
		{"empty input", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ProcessCommand(tc.input, nil)
			if got.BaseCmd != tc.want {
				t.Errorf("BaseCmd = %q, want %q (normalized: %q)", got.BaseCmd, tc.want, got.Normalized)
			}
		})
	}
}

func TestProcessCommand_Redaction(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantAbsent  []string
		wantPresent []string
		sameHashAs  string
	}{
		{
			name:        "sensitive env assign",
			input:       "TOKEN=hunter2 git status",
			wantAbsent:  []string{"hunter2"},
			wantPresent: []string{"TOKEN=REDACTED"},
			sameHashAs:  "TOKEN=different git status",
		},
		{
			name:        "flag equals form",
			input:       "curl --password=hunter2 http://example.com",
			wantAbsent:  []string{"hunter2"},
			wantPresent: []string{"--password=REDACTED"},
		},
		{
			name:        "flag space form",
			input:       "psql --password hunter2 mydb",
			wantAbsent:  []string{"hunter2"},
			wantPresent: []string{"--password REDACTED"},
		},
		{
			// redactURLCreds fires first (replacing user:secret@), then
			// redactEmails fires again on the REDACTED@ placeholder, leaving [EMAIL].
			name:        "url credentials",
			input:       "git clone https://user:s3cr3t@github.com/repo",
			wantAbsent:  []string{"s3cr3t", "user:s3cr3t"},
			wantPresent: []string{"[EMAIL]"},
		},
		{
			name:        "bearer token",
			input:       "curl -H 'Authorization: Bearer supersecrettoken'",
			wantAbsent:  []string{"supersecrettoken"},
			wantPresent: []string{"Bearer REDACTED"},
		},
		{
			name:        "long hex redacted",
			input:       "echo aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
			wantAbsent:  []string{"aabbccddeeff00112233445566778899"},
			wantPresent: []string{"[HEX"},
		},
		{
			name:        "short git sha preserved",
			input:       "git show abc1234",
			wantPresent: []string{"abc1234"},
		},
		{
			name:        "uuid redacted",
			input:       "docker rm f47ac10b-58cc-4372-a567-0e02b2c3d479",
			wantAbsent:  []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"},
			wantPresent: []string{"[UUID]"},
		},
		{
			name:        "email redacted",
			input:       "git log --author=user@example.com -n 5",
			wantAbsent:  []string{"user@example.com"},
			wantPresent: []string{"[EMAIL]"},
		},
		{
			name:        "private ip preserved",
			input:       "curl http://10.0.2.2:8080/health",
			wantPresent: []string{"10.0.2.2"},
		},
		{
			name:        "public ip redacted",
			input:       "curl http://8.8.8.8/query",
			wantAbsent:  []string{"8.8.8.8"},
			wantPresent: []string{"[PUBLIC IP]"},
		},
		{
			name:        "inline script normalized",
			input:       "python3 -c 'import sys; print(sys.argv)'",
			wantAbsent:  []string{"import sys"},
			wantPresent: []string{"[INLINE_SCRIPT"},
		},
		{
			name:        "long token normalized",
			input:       "cat " + strings.Repeat("x", 100),
			wantAbsent:  []string{strings.Repeat("x", 100)},
			wantPresent: []string{"[LONG STRING"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ProcessCommand(tc.input, nil)
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got.Normalized, absent) {
					t.Errorf("Normalized %q still contains %q", got.Normalized, absent)
				}
			}
			for _, present := range tc.wantPresent {
				if !strings.Contains(got.Normalized, present) {
					t.Errorf("Normalized %q missing %q", got.Normalized, present)
				}
			}
			if tc.sameHashAs != "" {
				other := ProcessCommand(tc.sameHashAs, nil)
				if got.Hash != other.Hash {
					t.Errorf("hash(%q)=%q, hash(%q)=%q: expected equal hashes",
						tc.input, got.Hash, tc.sameHashAs, other.Hash)
				}
			}
		})
	}
}

// TestProcessCommand_URLCredsCascade pins the two-pass cascade:
// redactURLCreds produces "REDACTED@host" which is itself a valid email
// address, so redactEmails consumes it in the next pass.
// The intermediate form must not survive into the final normalized output.
func TestProcessCommand_URLCredsCascade(t *testing.T) {
	const input = "git clone https://user:s3cr3t@github.com/repo"
	got := ProcessCommand(input, nil)

	absent := []string{
		"s3cr3t",              // original secret
		"user",                // original username
		"REDACTED@github.com", // intermediate form — must be consumed by email pass
	}
	for _, s := range absent {
		if strings.Contains(got.Normalized, s) {
			t.Errorf("Normalized %q still contains %q", got.Normalized, s)
		}
	}

	present := []string{
		"https://", // scheme preserved
		"[EMAIL]",  // cascade end-state
		"/repo",    // path preserved
	}
	for _, s := range present {
		if !strings.Contains(got.Normalized, s) {
			t.Errorf("Normalized %q missing %q", got.Normalized, s)
		}
	}
}

func TestProcessCommand_UserPattern(t *testing.T) {
	re := regexp.MustCompile(`acme-[a-z]+`)
	got := ProcessCommand("echo acme-secret", []*regexp.Regexp{re})
	if strings.Contains(got.Normalized, "acme-secret") {
		t.Errorf("user pattern not applied: %q", got.Normalized)
	}
	if !strings.Contains(got.Normalized, "[USER REDACTED]") {
		t.Errorf("expected [USER REDACTED] in %q", got.Normalized)
	}
}

func TestProcessCommand_RedactedByteCounts(t *testing.T) {
	// A 10-byte secret value in TOKEN= should produce a non-empty byte count entry.
	got := ProcessCommand("TOKEN=1234567890 git status", nil)
	if len(got.RedactedByteCounts) == 0 {
		t.Error("expected non-empty RedactedByteCounts for sensitive env assign")
	}
}
