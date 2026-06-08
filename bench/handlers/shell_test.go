package handlers

import (
	"testing"
)

func TestPromoteCWD(t *testing.T) {
	useCases := []struct {
		name      string
		cmd       string
		wantPath  string
		wantRem   string
	}{
		{
			name:     "plain cd prefix",
			cmd:      "cd /projects/foo && go build ./...",
			wantPath: "/projects/foo",
			wantRem:  "go build ./...",
		},
		{
			name:     "multi-segment chain",
			cmd:      "cd /projects/foo && go build ./... && go test ./...",
			wantPath: "/projects/foo",
			wantRem:  "go build ./... && go test ./...",
		},
		{
			name:     "no cd prefix",
			cmd:      "go build ./...",
			wantPath: "",
			wantRem:  "go build ./...",
		},
		{
			name:     "cd with no chain",
			cmd:      "cd /projects/foo",
			wantPath: "",
			wantRem:  "cd /projects/foo",
		},
		{
			name:     "quoted path skipped",
			cmd:      `cd "/path with spaces" && ls`,
			wantPath: "",
			wantRem:  `cd "/path with spaces" && ls`,
		},
		{
			name:     "path with space skipped",
			cmd:      "cd /path with spaces && ls",
			wantPath: "",
			wantRem:  "cd /path with spaces && ls",
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			gotPath, gotRem := promoteCWD(u.cmd)
			if gotPath != u.wantPath {
				t.Errorf("path: got %q, want %q", gotPath, u.wantPath)
			}
			if gotRem != u.wantRem {
				t.Errorf("remainder: got %q, want %q", gotRem, u.wantRem)
			}
		})
	}
}

func TestSplitOnAndAnd(t *testing.T) {
	useCases := []struct {
		name string
		cmd  string
		want []string
	}{
		{
			name: "no chain",
			cmd:  "go build ./...",
			want: []string{"go build ./..."},
		},
		{
			name: "two parts",
			cmd:  "go build ./... && go test ./...",
			want: []string{"go build ./...", "go test ./..."},
		},
		{
			name: "three parts",
			cmd:  "go build ./... && go test ./... && golangci-lint run ./...",
			want: []string{"go build ./...", "go test ./...", "golangci-lint run ./..."},
		},
		{
			name: "with 2>&1 redirects",
			cmd:  "go build ./... 2>&1 && go test ./... 2>&1",
			want: []string{"go build ./... 2>&1", "go test ./... 2>&1"},
		},
		{
			name: "&& inside single quotes not split",
			cmd:  "echo 'a && b' && ls",
			want: []string{"echo 'a && b'", "ls"},
		},
		{
			name: "&& inside double quotes not split",
			cmd:  `echo "a && b" && ls`,
			want: []string{`echo "a && b"`, "ls"},
		},
		{
			name: "escaped quote in double quotes",
			cmd:  `echo "he said \"hello && world\"" && ls`,
			want: []string{`echo "he said \"hello && world\""`, "ls"},
		},
		{
			name: "empty string",
			cmd:  "",
			want: []string{""},
		},
		{
			name: "single ampersand not split",
			cmd:  "cmd1 & cmd2",
			want: []string{"cmd1 & cmd2"},
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			got := splitOnAndAnd(u.cmd)
			if len(got) != len(u.want) {
				t.Fatalf("got %v, want %v", got, u.want)
			}
			for i := range got {
				if got[i] != u.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], u.want[i])
				}
			}
		})
	}
}
