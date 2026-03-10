package cmdlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/segmentio/ksuid"
)

// DataDir is the base directory for all repited data.
// C:\Users\<user>\AppData\Local\Repited
func DataDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", ".repited")
		}

		localAppData = filepath.Join(home, "AppData", "Local")
	}

	return filepath.Join(localAppData, "Repited")
}

// CommandsDir returns the path to the commands log directory.
func CommandsDir() string {
	return filepath.Join(DataDir(), "commands")
}

// DBPath returns the default SQLite database path inside the data directory.
func DBPath() string {
	return filepath.Join(DataDir(), "repited.db")
}

// Entry represents a single command that was executed.
type Entry struct {
	Cmd      string        // executable name
	Args     []string      // arguments
	Dir      string        // working directory
	Status   string        // "ok", "failed", "skipped", "warned"
	Duration time.Duration // how long it took
}

// Log represents a command log file for a single tool invocation.
type Log struct {
	ID      ksuid.KSUID
	Command string  // which repited command (flow, scan, etc.)
	Dir     string  // target directory
	Entries []Entry // commands executed
}

// New creates a new command log.
func New(command string, dir string) *Log {
	return &Log{
		ID:      ksuid.New(),
		Command: command,
		Dir:     dir,
	}
}

// Add records a command entry.
func (l *Log) Add(e Entry) {
	l.Entries = append(l.Entries, e)
}

// Save writes the log file to the commands directory.
// Format: {ksuid}_{command}.txt
func (l *Log) Save() (string, error) {
	dir := CommandsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating commands dir: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.txt", l.ID.String(), l.Command)
	path := filepath.Join(dir, filename)

	content := l.Format()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("writing log file: %w", err)
	}

	return path, nil
}

// Format renders the log content as a clean, parseable text format.
//
// Example output:
//
//	# repited flow
//	# dir: D:\shared\development\personal\projects\repited
//	# time: 2026-03-09T21:30:00Z
//	# id: 2QHvBm8VxZQj5K0VzEg3N1uUdJf
//
//	[ok 1.2s] go mod tidy
//	[ok 0.8s] go build ./...
//	[ok 0.5s] go vet ./...
//	[warn 4.3s] go test -count=1 ./...
//	[skip] golangci-lint run ./...
//	[ok 0.1s] git add .
//	[ok 0.0s] git status --short
//	[ok 0.2s] git commit -m "feat: add flow command"
func (l *Log) Format() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# repited %s\n", l.Command)
	fmt.Fprintf(&b, "# dir: %s\n", l.Dir)
	fmt.Fprintf(&b, "# time: %s\n", l.ID.Time().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "# id: %s\n", l.ID.String())
	b.WriteString("\n")

	for _, e := range l.Entries {
		cmdLine := e.Cmd
		if len(e.Args) > 0 {
			cmdLine += " " + shelljoin(e.Args)
		}

		switch e.Status {
		case "skipped":
			fmt.Fprintf(&b, "[skip] %s\n", cmdLine)
		default:
			fmt.Fprintf(&b, "[%s %.1fs] %s\n", e.Status, e.Duration.Seconds(), cmdLine)
		}
	}

	return b.String()
}

// shelljoin quotes arguments that contain spaces.
func shelljoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"'\\") {
			parts[i] = fmt.Sprintf("%q", a)
		} else {
			parts[i] = a
		}
	}

	return strings.Join(parts, " ")
}
