// Package wps implements WPS 1.0 handlers and the async job store.
package wps

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/geoson/geoson/services/wps/internal/process"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type JobStatus struct {
	ID      string `json:"id"`
	Process string `json:"process"`
	Status  string `json:"status"` // accepted | running | succeeded | failed
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

type jobMsg struct {
	ID      string            `json:"id"`
	Process string            `json:"process"`
	Inputs  map[string]string `json:"inputs"`
}

type Jobs struct {
	dir string
	nc  *nats.Conn
	db  *pgxpool.Pool
}

const subject = "wps.jobs"

func NewJobs(dir string, nc *nats.Conn, db *pgxpool.Pool) *Jobs {
	os.MkdirAll(dir, 0o755)
	return &Jobs{dir: dir, nc: nc, db: db}
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (j *Jobs) statusPath(id string) string { return filepath.Join(j.dir, id+".json") }

func (j *Jobs) writeStatus(st JobStatus) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(j.statusPath(st.ID), data, 0o644)
}

func (j *Jobs) Status(id string) (JobStatus, error) {
	data, err := os.ReadFile(j.statusPath(id))
	if err != nil {
		return JobStatus{}, err
	}
	var st JobStatus
	return st, json.Unmarshal(data, &st)
}

// execNow runs a process synchronously.
func (j *Jobs) execNow(ctx context.Context, procID string, inputs map[string]string) (string, error) {
	p, ok := process.Get(procID)
	if !ok {
		return "", fmt.Errorf("unknown process %q", procID)
	}
	return p.Run(ctx, j.db, inputs)
}

// Enqueue creates a job (status accepted), publishes it, returns the id.
func (j *Jobs) Enqueue(ctx context.Context, procID string, inputs map[string]string) (string, error) {
	id := newID()
	if err := j.writeStatus(JobStatus{ID: id, Process: procID, Status: "accepted"}); err != nil {
		return "", err
	}
	if j.nc != nil {
		msg, _ := json.Marshal(jobMsg{ID: id, Process: procID, Inputs: inputs})
		if err := j.nc.Publish(subject, msg); err != nil {
			return "", err
		}
	}
	return id, nil
}

// RunWorker subscribes to the job queue and executes jobs until ctx is done.
func (j *Jobs) RunWorker(ctx context.Context) error {
	if j.nc == nil {
		return fmt.Errorf("no nats connection")
	}
	sub, err := j.nc.Subscribe(subject, func(m *nats.Msg) {
		var job jobMsg
		if json.Unmarshal(m.Data, &job) != nil {
			return
		}
		j.writeStatus(JobStatus{ID: job.ID, Process: job.Process, Status: "running"})
		out, err := j.execNow(context.Background(), job.Process, job.Inputs)
		if err != nil {
			j.writeStatus(JobStatus{ID: job.ID, Process: job.Process, Status: "failed", Error: err.Error()})
			return
		}
		j.writeStatus(JobStatus{ID: job.ID, Process: job.Process, Status: "succeeded", Output: out})
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	sub.Unsubscribe()
	return nil
}
