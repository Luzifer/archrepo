package status

import (
	"fmt"
	"io"
	"os"
	"slices"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiGrey   = "\x1b[90m"
	ansiGreen  = "\x1b[32m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
)

type (
	// Display tracks jobs and renders their current state either as a live TTY
	// view on stdout or as fallback logrus entries in non-interactive mode.
	Display struct {
		mu        sync.Mutex
		out       io.Writer
		isTTY     bool
		jobs      []job
		jobIndex  map[string]int
		lineCount int
	}

	// JobStatus represents the state of a tracked job.
	JobStatus uint

	job struct {
		identifier string
		status     JobStatus
		comment    string
	}
)

// Enum defininig the available JobStatus
const (
	JobStatusUnknown JobStatus = iota // Not to be used, Error-State!
	JobStatusRunning
	JobStatusSkipped
	JobStatusSuccess
	JobStatusFailure
)

var finalStatus = []JobStatus{JobStatusSkipped, JobStatusSuccess, JobStatusFailure}

// NewDisplay creates a display that auto-detects whether stdout is a TTY.
func NewDisplay() *Display {
	return &Display{
		out:      os.Stdout,
		isTTY:    isTerminal(os.Stdout),
		jobIndex: make(map[string]int),
	}
}

func formatJobLine(job job) string {
	line := fmt.Sprintf("%s %s%s%s", statusColor(job.status)+statusIcon(job.status)+ansiReset, ansiBold, job.identifier, ansiReset)
	if job.comment == "" {
		return line
	}

	return line + " " + job.comment
}

func statusColor(status JobStatus) string {
	switch status {
	case JobStatusSkipped:
		return ansiGrey
	case JobStatusSuccess:
		return ansiGreen
	case JobStatusFailure:
		return ansiRed
	case JobStatusRunning:
		return ansiYellow
	default:
		return ""
	}
}

func statusIcon(status JobStatus) string {
	switch status {
	case JobStatusSkipped:
		return "󰜺"
	case JobStatusSuccess:
		return "󰄬"
	case JobStatusFailure:
		return "󰅚"
	case JobStatusRunning:
		return "󰔟"
	default:
		return "󰋗"
	}
}

func isFinal(status JobStatus) bool {
	return slices.Contains(finalStatus, status)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}

// AddJob registers a job in creation order with its current status and comment.
func (d *Display) AddJob(identifier string, status JobStatus, comment string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if idx, ok := d.jobIndex[identifier]; ok {
		d.updateJobLocked(idx, status, comment)
		return
	}

	d.jobIndex[identifier] = len(d.jobs)
	d.jobs = append(d.jobs, job{
		identifier: identifier,
		status:     status,
		comment:    comment,
	})

	if d.isTTY {
		d.renderLocked()
		return
	}

	d.logJobChangeLocked(job{}, d.jobs[len(d.jobs)-1], true)
}

func (s JobStatus) String() string {
	switch s {
	case JobStatusRunning:
		return "running"
	case JobStatusSkipped:
		return "skipped"
	case JobStatusSuccess:
		return "success"
	case JobStatusFailure:
		return "failure"
	default:
		return "unknown"
	}
}

// UpdateJob updates a tracked job to the provided status and comment.
func (d *Display) UpdateJob(identifier string, status JobStatus, comment string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if idx, ok := d.jobIndex[identifier]; ok {
		d.updateJobLocked(idx, status, comment)
		return
	}

	d.jobIndex[identifier] = len(d.jobs)
	d.jobs = append(d.jobs, job{
		identifier: identifier,
		status:     status,
		comment:    comment,
	})

	if d.isTTY {
		d.renderLocked()
		return
	}

	d.logJobChangeLocked(job{}, d.jobs[len(d.jobs)-1], true)
}

func (*Display) logJobChangeLocked(before, after job, added bool) {
	entry := logrus.WithFields(logrus.Fields{
		"package": after.identifier,
		"status":  after.status.String(),
	})
	if after.comment != "" {
		entry = entry.WithField("comment", after.comment)
	}

	switch {
	case added && after.status == JobStatusRunning:
		entry.Info("job started")
	case isFinal(after.status):
		if after.status == JobStatusFailure {
			entry.Error("job failed")
			return
		}
		entry.Info("job completed")
	case before.status != after.status || before.comment != after.comment:
		entry.Debug("job update")
	}
}

func (d *Display) renderLocked() {
	if d.lineCount > 0 {
		_, _ = fmt.Fprintf(d.out, "\x1b[%dA", d.lineCount)
	}

	for _, job := range d.jobs {
		_, _ = fmt.Fprintf(d.out, "\r\x1b[2K%s", formatJobLine(job))
		_, _ = fmt.Fprint(d.out, "\n")
	}

	d.lineCount = len(d.jobs)
}

func (d *Display) updateJobLocked(idx int, status JobStatus, comment string) {
	before := d.jobs[idx]
	after := before
	after.status = status
	after.comment = comment

	if before.status == after.status && before.comment == after.comment {
		return
	}

	d.jobs[idx] = after

	if d.isTTY {
		d.renderLocked()
		return
	}

	d.logJobChangeLocked(before, after, false)
}
