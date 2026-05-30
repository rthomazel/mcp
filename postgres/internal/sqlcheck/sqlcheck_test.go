package sqlcheck

import (
	"testing"
)

func TestStripComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "line comment",
			input:    "SELECT * FROM users -- get all users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "block comment",
			input:    "SELECT /* all columns */ * FROM users",
			expected: "SELECT  * FROM users",
		},
		{
			name:     "trailing semicolon preserved",
			input:    "SELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			name:     "multiline with line comments",
			input:    "SELECT *\nFROM users -- table",
			expected: "SELECT *\nFROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripComments(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		allowlist []string
		wantErr   bool
		errMsg    string
		wantSQL   string
	}{
		{
			name:      "valid SELECT",
			input:     "SELECT * FROM users",
			allowlist: []string{"SELECT"},
			wantSQL:   "SELECT * FROM users",
		},
		{
			name:      "case insensitive input",
			input:     "select * from users",
			allowlist: []string{"SELECT"},
			wantSQL:   "select * from users",
		},
		{
			name:      "valid WITH CTE",
			input:     "WITH cte AS (SELECT 1) SELECT * FROM cte",
			allowlist: []string{"SELECT", "WITH"},
			wantSQL:   "WITH cte AS (SELECT 1) SELECT * FROM cte",
		},
		{
			name:      "trailing semicolon is fine",
			input:     "SELECT 1;",
			allowlist: []string{"SELECT"},
			wantSQL:   "SELECT 1;",
		},
		{
			name:      "multiple statements rejected",
			input:     "SELECT 1; SELECT 2",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "multiple statements are not allowed",
		},
		{
			name:      "multiple statements with trailing semicolon rejected",
			input:     "SELECT 1; SELECT 2;",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "multiple statements are not allowed",
		},
		{
			name:      "BEGIN rejected",
			input:     "BEGIN",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "transaction-control statements are not allowed",
		},
		{
			name:      "COMMIT rejected",
			input:     "COMMIT",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "transaction-control statements are not allowed",
		},
		{
			name:      "ROLLBACK rejected",
			input:     "ROLLBACK",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "transaction-control statements are not allowed",
		},
		{
			name:      "SAVEPOINT rejected",
			input:     "SAVEPOINT sp1",
			allowlist: []string{"SELECT"},
			wantErr:   true,
			errMsg:    "transaction-control statements are not allowed",
		},
		{
			name:      "wrong SQL class rejected",
			input:     "INSERT INTO users VALUES (1)",
			allowlist: []string{"SELECT", "WITH"},
			wantErr:   true,
			errMsg:    `statement type "INSERT" is not allowed by this tool`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Validate(tt.input, tt.allowlist)
			if (err != nil) != tt.wantErr {
				t.Errorf("error mismatch: got %v, wantErr=%v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("error message: got %q, want %q", err.Error(), tt.errMsg)
			}
			if !tt.wantErr && result != tt.wantSQL {
				t.Errorf("SQL: got %q, want %q", result, tt.wantSQL)
			}
		})
	}
}
