package handlers

import (
	"strings"
	"testing"
)

func TestParseMounts(t *testing.T) {
	useCases := []struct {
		name  string
		input string
		want  []mount
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name: "siblings kept",
			input: `/dev/sda1 /projects/foo ext4 rw 0 0
/dev/sda1 /projects/bar ext4 rw 0 0
`,
			want: []mount{
				{mountpoint: "/projects/bar", ro: false, persistent: false},
				{mountpoint: "/projects/foo", ro: false, persistent: false},
			},
		},
		{
			name: "submounts kept",
			input: `/dev/sda1 /projects/foo ext4 rw 0 0
/dev/sda1 /projects/foo/.git ext4 ro 0 0
`,
			want: []mount{
				{mountpoint: "/projects/foo", ro: false, persistent: false},
				{mountpoint: "/projects/foo/.git", ro: true, persistent: false},
			},
		},
		{
			name: "persistent volumes",
			input: `/dev/sda1 /mise ext4 rw 0 0
/dev/sda1 /root ext4 rw 0 0
`,
			want: []mount{
				{mountpoint: "/mise", ro: false, persistent: true},
				{mountpoint: "/root", ro: false, persistent: true},
			},
		},
		{
			name: "skip noise fstypes",
			input: `/dev/sda1 /projects/foo ext4 rw 0 0
proc /proc proc rw 0 0
tmpfs /tmp tmpfs rw 0 0
`,
			want: []mount{
				{mountpoint: "/projects/foo", ro: false, persistent: false},
			},
		},
		{
			name: "skip /etc mounts",
			input: `/dev/sda1 /projects/foo ext4 rw 0 0
/dev/sda1 /etc/hosts ext4 rw 0 0
`,
			want: []mount{
				{mountpoint: "/projects/foo", ro: false, persistent: false},
			},
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			got, err := parseMounts(strings.NewReader(u.input), "/root", "/mise")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(u.want) {
				t.Fatalf("got %v, want %v", got, u.want)
			}
			for i := range got {
				if got[i] != u.want[i] {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], u.want[i])
				}
			}
		})
	}
}
