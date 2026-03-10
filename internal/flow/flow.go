package flow

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Step represents a single step in a workflow.
type Step struct {
	Name    string   // display name
	Cmd     string   // executable
	Args    []string // arguments
	Dir     string   // working directory
	Skip    bool     // skip this step
	OnFail  string   // "stop" (default), "warn", "skip"
	Require string   // file/dir that must exist for step to run (empty = always)
}

// Result holds the outcome of a single step.
type Result struct {
	Step     Step
	Status   string // "ok", "failed", "skipped", "warned"
	Output   string
	Duration time.Duration
	Err      error
}

// Pipeline runs steps sequentially, stopping on first failure unless OnFail says otherwise.
type Pipeline struct {
	Steps   []Step
	Results []Result
	Dir     string
	Verbose bool
	Quiet   bool // suppress stdout output (for MCP/log-only mode)
}

// NewPipeline creates a pipeline with a working directory.
func NewPipeline(dir string, verbose bool) *Pipeline {
	return &Pipeline{Dir: dir, Verbose: verbose}
}

// Add appends a step.
func (p *Pipeline) Add(s Step) {
	if s.Dir == "" {
		s.Dir = p.Dir
	}

	if s.OnFail == "" {
		s.OnFail = "stop"
	}

	p.Steps = append(p.Steps, s)
}

// Run executes all steps in order.
func (p *Pipeline) Run() error {
	total := 0

	for _, s := range p.Steps {
		if !s.Skip {
			total++
		}
	}

	current := 0

	for _, step := range p.Steps {
		if step.Skip {
			p.Results = append(p.Results, Result{
				Step:   step,
				Status: "skipped",
			})

			continue
		}

		// Check requirement
		if step.Require != "" {
			path := step.Require
			if !strings.HasPrefix(path, "/") && !strings.Contains(path, ":") {
				path = step.Dir + "/" + path
			}

			if _, err := os.Stat(path); os.IsNotExist(err) {
				p.Results = append(p.Results, Result{
					Step:   step,
					Status: "skipped",
					Output: fmt.Sprintf("requires %s", step.Require),
				})

				continue
			}
		}

		current++
		if !p.Quiet {
			_, _ = fmt.Fprintf(os.Stdout, "  [%d/%d] %s ", current, total, step.Name)
		}

		start := time.Now()
		output, err := runStep(step)
		duration := time.Since(start)

		result := Result{
			Step:     step,
			Output:   output,
			Duration: duration,
			Err:      err,
		}

		if err != nil {
			switch step.OnFail {
			case "warn":
				result.Status = "warned"

				if !p.Quiet {
					_, _ = fmt.Fprintf(os.Stdout, "⚠ %.1fs (warning, continuing)\n", duration.Seconds())

					if p.Verbose {
						printIndented(output)
					}
				}
			case "skip":
				result.Status = "skipped"

				if !p.Quiet {
					_, _ = fmt.Fprintf(os.Stdout, "→ %.1fs (skipped)\n", duration.Seconds())
				}
			default: // "stop"
				result.Status = "failed"

				if !p.Quiet {
					_, _ = fmt.Fprintf(os.Stdout, "✘ %.1fs\n", duration.Seconds())

					printIndented(output)
				}

				p.Results = append(p.Results, result)

				return fmt.Errorf("step %q failed: %w", step.Name, err)
			}
		} else {
			result.Status = "ok"

			if !p.Quiet {
				_, _ = fmt.Fprintf(os.Stdout, "✓ %.1fs\n", duration.Seconds())

				if p.Verbose {
					printIndented(output)
				}
			}
		}

		p.Results = append(p.Results, result)
	}

	return nil
}

// Summary prints a summary of all results.
func (p *Pipeline) Summary() {
	_, _ = fmt.Fprintln(os.Stdout)
	_, _ = fmt.Fprintln(os.Stdout, "  ─── Summary ───")

	var totalDuration time.Duration

	ok, failed, skipped, warned := 0, 0, 0, 0

	for _, r := range p.Results {
		totalDuration += r.Duration
		switch r.Status {
		case "ok":
			ok++
		case "failed":
			failed++
		case "skipped":
			skipped++
		case "warned":
			warned++
		}
	}

	_, _ = fmt.Fprintf(os.Stdout, "  %d passed", ok)

	if warned > 0 {
		_, _ = fmt.Fprintf(os.Stdout, ", %d warned", warned)
	}

	if skipped > 0 {
		_, _ = fmt.Fprintf(os.Stdout, ", %d skipped", skipped)
	}

	if failed > 0 {
		_, _ = fmt.Fprintf(os.Stdout, ", %d failed", failed)
	}

	_, _ = fmt.Fprintf(os.Stdout, " (%.1fs total)\n", totalDuration.Seconds())
}

func runStep(step Step) (string, error) {
	cmd := exec.Command(step.Cmd, step.Args...)
	cmd.Dir = step.Dir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()

	return string(out), err
}

func printIndented(output string) {
	if output == "" {
		return
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	limit := 20
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
		_, _ = fmt.Fprintf(os.Stdout, "         ... (showing last %d lines)\n", limit)
	}

	for _, line := range lines {
		_, _ = fmt.Fprintf(os.Stdout, "         %s\n", line)
	}
}
