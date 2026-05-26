package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/getcourse-downloader/getcourse-downloader/internal/bundle"
	"github.com/getcourse-downloader/getcourse-downloader/internal/config"
	"github.com/getcourse-downloader/getcourse-downloader/internal/download"
)

type Status struct {
	ID        string                `json:"id"`
	State     string                `json:"state"`
	Progress  int                   `json:"progress"`
	Message   string                `json:"message"`
	Result    *bundle.ProcessResult `json:"result,omitempty"`
	Error     string                `json:"error,omitempty"`
	StartedAt time.Time             `json:"started_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

type jobRequest struct {
	id      string
	cfg     *config.Config
	payload *bundle.LessonPayload
}

type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*Status

	once    sync.Once
	queue   chan jobRequest
	workers int
}

var Default = &Manager{jobs: make(map[string]*Status)}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) ensureWorkers(maxParallel int) {
	m.once.Do(func() {
		if maxParallel < 1 {
			maxParallel = 2
		}
		m.workers = maxParallel
		m.queue = make(chan jobRequest, 256)
		for i := 0; i < m.workers; i++ {
			go m.worker()
		}
	})
}

func (m *Manager) worker() {
	for req := range m.queue {
		m.runJob(req)
	}
}

func (m *Manager) runJob(req jobRequest) {
	result, err := download.ProcessLessonWithProgress(req.cfg, req.payload, func(pct int, msg string) {
		m.update(req.id, "running", pct, msg, nil, "")
	})
	if err != nil {
		m.update(req.id, "error", 100, err.Error(), nil, err.Error())
		return
	}
	m.update(req.id, "done", 100, "Готово", result, "")
}

func (m *Manager) Start(cfg *config.Config, payload *bundle.LessonPayload) string {
	m.ensureWorkers(cfg.MaxParallelJobs)

	id := newID()
	st := &Status{
		ID:        id,
		State:     "running",
		Progress:  0,
		Message:   "Старт…",
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[id] = st
	m.mu.Unlock()

	m.queue <- jobRequest{id: id, cfg: cfg, payload: payload}
	return id
}

func (m *Manager) update(id, state string, progress int, message string, result *bundle.ProcessResult, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.jobs[id]
	if !ok {
		return
	}
	st.State = state
	st.Progress = progress
	st.Message = message
	st.Result = result
	st.Error = errMsg
	st.UpdatedAt = time.Now()
}

func (m *Manager) Get(id string) *Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st, ok := m.jobs[id]
	if !ok {
		return nil
	}
	cp := *st
	return &cp
}
