package settings

import (
	"sync"

	"syslog-analytics-mvp/internal/config"
)

type Runtime struct {
	mu        sync.RWMutex
	retention config.Retention
}

func New(initial config.Retention) *Runtime {
	return &Runtime{retention: initial}
}

func (r *Runtime) Retention() config.Retention {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retention
}

func (r *Runtime) UpdateRetention(next config.Retention) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retention = next
}
