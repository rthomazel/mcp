package stats

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadKeyFrom(t *testing.T) {
	validKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))

	cases := []struct {
		name    string
		content *string
		wantKey bool
		wantErr bool
	}{
		{name: "missing file", content: nil},
		{name: "empty file", content: ptr("")},
		{name: "whitespace only", content: ptr(" \n\t\n")},
		{name: "valid key", content: ptr(validKey + "\n"), wantKey: true},
		{name: "invalid base64", content: ptr("not base64!!"), wantErr: true},
		{name: "wrong length", content: ptr(base64.StdEncoding.EncodeToString([]byte("short"))), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key")
			if tc.content != nil {
				if err := os.WriteFile(path, []byte(*tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			key, err := loadKeyFrom(path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if (len(key) > 0) != tc.wantKey {
				t.Errorf("key present = %v, want %v", len(key) > 0, tc.wantKey)
			}
		})
	}
}

func ptr(s string) *string { return &s }
