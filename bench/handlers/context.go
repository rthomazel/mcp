package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/pathsnapshot"
	"github.com/samber/lo"
)

var (
	skipFSTypes  = []string{"proc", "sysfs", "tmpfs", "devpts", "cgroup2", "cgroup", "mqueue", "overlay"}
	skipPrefixes = []string{"/proc", "/sys", "/dev", "/run", "/etc"}
)

var preInstalled = map[string]string{
	"bash":    "bash --version | head -1 | cut -d' ' -f4",
	"git":     "git --version | cut -d' ' -f3",
	"jujutsu": "jj version",
	"mise":    "mise v 2>/dev/null | tail -1",
	"python3": "python3 --version | cut -d' ' -f2",
	"pip3":    "pip3 --version 2>/dev/null | cut -d' ' -f2",
	"rg":      "rg --version | head -1 | cut -d' ' -f2",
	"make":    "make --version | head -1 | cut -d' ' -f3",
	"jq":      "jq --version",
	"curl":    "curl --version | head -1 | cut -d' ' -f1-2",
}

type mount struct {
	mountpoint string
	ro         bool
	persistent bool
}

func (h *Handler) HandleContext(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	gather := func(cmd string) string {
		r := runCommand(ctx, h.cfg, cmd, "/")
		return strings.TrimSpace(r.Stdout)
	}

	osName := gather("cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"'")
	arch := gather("uname -m")
	disk := gather("df -h / | awk 'NR==2{print $4\" free of \"$2}'")
	path := os.Getenv("PATH")

	var mounts []mount
	file, err := os.Open("/proc/mounts")
	if err != nil {
		slog.Error("failed to read mounts", "err", err)
	} else {
		defer func() { _ = file.Close() }()
		mounts, err = parseMounts(file, h.cfg.Home, h.cfg.MiseDir)
		if err != nil {
			slog.Error("failed to parse mounts", "err", err)
		}
	}

	versions := make(map[string]string, len(preInstalled))
	for name, cmd := range preInstalled {
		v := gather(cmd)
		if v == "" {
			v = "-"
		}
		versions[name] = v
	}

	detected := pathsnapshot.Diff(h.cfg.Home)

	return mcp.NewToolResultText(formatPlainTextContext(osName, arch, disk, path, h.cfg.Timeout.String(), h.version, h.cfg.Home, mounts, versions, detected)), nil
}

func formatPlainTextContext(osName, arch, disk, path, timeout, version, home string, mounts []mount, versions map[string]string, detected []pathsnapshot.Entry) string {
	b := strings.Builder{}

	b.WriteString("<metadata>\n")
	b.WriteString("os: " + osName + "\n")
	b.WriteString("arch: " + arch + "\n")
	b.WriteString("disk: " + disk + "\n")
	b.WriteString("path: " + path + "\n")
	b.WriteString("shell_exec_timeout: " + timeout + "\n")
	b.WriteString("version: " + version + "\n")

	b.WriteString("volumes:\n")
	for _, m := range mounts {
		switch {
		case m.persistent:
			b.WriteString("  " + m.mountpoint + " persistent\n")
		case m.ro:
			b.WriteString("  " + m.mountpoint + " ro\n")
		default:
			b.WriteString("  " + m.mountpoint + " rw\n")
		}
	}
	b.WriteString("note: container is ephemeral — only volumes above persist across sessions; install to " + home + "/bin to persist across sessions\n")

	b.WriteString("installed:\n")
	maxLen := 0

	for name := range preInstalled {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	for name := range preInstalled {
		b.WriteString("  " + fmt.Sprintf("%-*s", maxLen+1, name+":") + " " + versions[name] + "\n")
	}

	if len(detected) > 0 {
		b.WriteString("auto-detected in path:\n")
		for _, e := range detected {
			b.WriteString("  " + e.Name + " " + e.Path + "\n")
		}
	}

	b.WriteString("</metadata>\n")

	return b.String()
}

func parseMounts(r io.Reader, home, miseDir string) ([]mount, error) {
	var mounts []mount
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}

		mountpoint, fstype, options := fields[1], fields[2], fields[3]

		if mountpoint == "/" || lo.Contains(skipFSTypes, fstype) {
			continue
		}

		isSkipped := lo.SomeBy(skipPrefixes, func(p string) bool {
			return mountpoint == p || strings.HasPrefix(mountpoint, p+"/")
		})
		if isSkipped {
			continue
		}

		ro := strings.HasPrefix(options, "ro,") || options == "ro"
		persistent := lo.Contains([]string{miseDir, home}, mountpoint)

		mounts = append(mounts, mount{
			mountpoint: mountpoint,
			ro:         ro,
			persistent: persistent,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].mountpoint < mounts[j].mountpoint
	})

	return mounts, nil
}
