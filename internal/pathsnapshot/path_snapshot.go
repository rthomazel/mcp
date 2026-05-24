package pathsnapshot

import (
	"bufio"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const snapshotFile = ".bench-mcp-path-snapshot"

// Entry is a single executable discovered in PATH.
type Entry struct {
	Name string
	Path string
}

// Diff rescans PATH and returns executables not present in the snapshot.
// If the snapshot does not exist it is created and nil is returned —
// nothing to diff on first run.
// home is the base directory for the snapshot file (e.g. cfg.Home).
func Diff(home string) []Entry {
	snapshotPath := filepath.Join(home, snapshotFile)
	current := scan()
	snapshot, err := load(snapshotPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("path snapshot diff", "err", err.Error())
			return nil
		}

		_ = write(current, snapshotPath)
		return nil
	}

	return diff(snapshot, current)
}

// scan walks every directory in $PATH and returns a sorted slice of
// executables. The same name appearing in multiple PATH dirs produces
// multiple entries — each is a distinct binary at a distinct path.
func scan() []Entry {
	dirs := strings.Split(os.Getenv("PATH"), ":")
	seen := map[string]struct{}{}
	var entries []Entry

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		infos, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, info := range infos {
			if info.IsDir() {
				continue
			}
			fi, err := info.Info()
			if err != nil || fi.Mode()&0o111 == 0 {
				continue
			}
			full := filepath.Join(dir, info.Name())
			if _, dup := seen[full]; dup {
				continue
			}
			seen[full] = struct{}{}
			entries = append(entries, Entry{Name: info.Name(), Path: full})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Path < entries[j].Path
	})

	return entries
}

// write atomically writes entries to snapshotPath as TSV (name\tpath).
func write(entries []Entry, snapshotPath string) error {
	tmp := snapshotPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	for _, e := range entries {
		_, _ = w.WriteString(e.Name + "\t" + e.Path + "\n")
	}

	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, snapshotPath)
}

// load reads snapshotPath and returns the stored entries.
// Returns fs.ErrNotExist if the file has not been written yet.
func load(snapshotPath string) ([]Entry, error) {
	f, err := os.Open(snapshotPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, path, ok := strings.Cut(scanner.Text(), "\t")
		if !ok || name == "" {
			continue
		}
		entries = append(entries, Entry{Name: name, Path: path})
	}

	return entries, scanner.Err()
}

// diff returns entries present in current but absent from snapshot.
// Both slices must be sorted by Name then Path.
func diff(snapshot, current []Entry) []Entry {
	var result []Entry
	si := 0

	for _, c := range current {
		for si < len(snapshot) && entryLess(snapshot[si], c) {
			si++
		}
		if si < len(snapshot) && snapshot[si] == c {
			continue
		}
		result = append(result, c)
	}

	return result
}

func entryLess(a, b Entry) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Path < b.Path
}
