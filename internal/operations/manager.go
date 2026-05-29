package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/store"
	"github.com/google/uuid"
)

type Manager struct {
	db      *store.DB
	baseCtx context.Context
}

type Spec struct {
	Kind          string
	ResourceType  string
	ResourceID    string
	ResourceName  string
	ActorUserID   string
	ActorUsername string
	Metadata      map[string]any
	Timeout       time.Duration
}

type Run struct {
	db          *store.DB
	operationID string
	mu          sync.Mutex
	nextPos     int
	summary     string
}

type Step struct {
	db          *store.DB
	id          string
	operationID string
}

func NewManager(baseCtx context.Context, db *store.DB) *Manager {
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	return &Manager{db: db, baseCtx: baseCtx}
}

func (m *Manager) Start(spec Spec, fn func(context.Context, *Run) error) (*store.Operation, error) {
	op, run, err := m.Begin(spec)
	if err != nil {
		return nil, err
	}

	go func() {
		timeout := spec.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Minute
		}
		ctx, cancel := context.WithTimeout(m.baseCtx, timeout)
		defer cancel()

		_ = m.Finish(run, spec, fn(ctx, run))
	}()

	return m.db.GetOperation(op.ID)
}

func (m *Manager) Begin(spec Spec) (*store.Operation, *Run, error) {
	if spec.Kind == "" {
		return nil, nil, fmt.Errorf("operation kind is required")
	}
	if spec.ResourceType == "" {
		spec.ResourceType = "system"
	}
	metadata, err := json.Marshal(spec.Metadata)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal operation metadata: %w", err)
	}

	now := time.Now().UTC()
	op := &store.Operation{
		ID:            uuid.NewString(),
		Kind:          spec.Kind,
		ResourceType:  spec.ResourceType,
		ResourceID:    spec.ResourceID,
		ResourceName:  spec.ResourceName,
		Status:        store.OperationStatusQueued,
		ActorUserID:   spec.ActorUserID,
		ActorUsername: spec.ActorUsername,
		Metadata:      metadata,
		CreatedAt:     now,
	}
	if err := m.db.CreateOperation(op); err != nil {
		return nil, nil, err
	}
	if err := m.db.SetOperationRunning(op.ID, now); err != nil {
		return nil, nil, err
	}

	run := &Run{
		db:          m.db,
		operationID: op.ID,
		summary:     strings.ReplaceAll(spec.Kind, "_", " "),
	}
	stored, err := m.db.GetOperation(op.ID)
	if err != nil {
		return nil, nil, err
	}
	return stored, run, nil
}

func (m *Manager) Finish(run *Run, spec Spec, err error) error {
	finishedAt := time.Now().UTC()
	summary := run.summary
	if summary == "" {
		summary = strings.ReplaceAll(spec.Kind, "_", " ")
	}
	if err != nil {
		if dbErr := m.db.FailOperation(run.operationID, summary, err.Error(), finishedAt); dbErr != nil {
			logger.Errorf("failed to mark operation %s failed: %v", run.operationID, dbErr)
			return dbErr
		}
		return err
	}
	if dbErr := m.db.CompleteOperation(run.operationID, summary, finishedAt); dbErr != nil {
		logger.Errorf("failed to mark operation %s complete: %v", run.operationID, dbErr)
		return dbErr
	}
	return nil
}

func (r *Run) ID() string {
	return r.operationID
}

func (r *Run) SetSummary(summary string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summary = strings.TrimSpace(summary)
	_ = r.db.SetOperationSummary(r.operationID, r.summary)
}

func (r *Run) BindResource(resourceType, resourceID, resourceName string) {
	if resourceType == "" {
		resourceType = "system"
	}
	if err := r.db.UpdateOperationResource(r.operationID, resourceType, resourceID, resourceName); err != nil {
		logger.Errorf("failed to bind resource for operation %s: %v", r.operationID, err)
	}
}

func (r *Run) Step(ctx context.Context, key, title string, fn func(context.Context, *Step) error) error {
	if title == "" {
		title = strings.ReplaceAll(key, "_", " ")
	}
	now := time.Now().UTC()

	r.mu.Lock()
	r.nextPos++
	position := r.nextPos
	r.mu.Unlock()

	step := &store.OperationStep{
		ID:          uuid.NewString(),
		OperationID: r.operationID,
		Position:    position,
		StepKey:     key,
		Title:       title,
		Status:      store.OperationStepStatusPending,
		CreatedAt:   now,
	}
	if err := r.db.CreateOperationStep(step); err != nil {
		return err
	}
	if err := r.db.StartOperationStep(step.ID, now); err != nil {
		return err
	}

	active := &Step{db: r.db, id: step.ID, operationID: r.operationID}
	err := fn(ctx, active)
	finishedAt := time.Now().UTC()
	if err != nil {
		_ = r.db.FinishOperationStep(step.ID, store.OperationStepStatusFailed, err.Error(), finishedAt)
		return err
	}
	if err := r.db.FinishOperationStep(step.ID, store.OperationStepStatusSucceeded, "", finishedAt); err != nil {
		return err
	}
	return nil
}

func (s *Step) Logf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		return
	}
	if err := s.db.AppendOperationStepOutput(s.id, msg); err != nil {
		logger.Errorf("failed to append operation output for step %s: %v", s.id, err)
	}
}

func (s *Step) Writer() io.Writer {
	return &stepWriter{step: s}
}

type stepWriter struct {
	step *Step
	mu   sync.Mutex
	buf  bytes.Buffer
}

func (w *stepWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(p)
	if _, err := w.buf.Write(p); err != nil {
		return 0, err
	}

	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Put partial line back for the next write.
			if line != "" {
				rest := []byte(line)
				remaining := append(rest, w.buf.Bytes()...)
				w.buf.Reset()
				_, _ = w.buf.Write(remaining)
			}
			break
		}
		w.step.Logf("%s", strings.TrimSpace(line))
	}
	return total, nil
}

func (w *stepWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() == 0 {
		return
	}
	w.step.Logf("%s", strings.TrimSpace(w.buf.String()))
	w.buf.Reset()
}
