// Package crud implements the minimal REST CRUD service, backed by an in-memory
// store, that powers the crud-api binary. It owns the resource handlers and is
// the source of the application's log activity.
package crud

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	mu     sync.RWMutex
	tasks  map[string]Task
	nextID int
	logger *slog.Logger
}

func New(logger *slog.Logger) *Service {
	return &Service{
		tasks:  make(map[string]Task),
		logger: logger,
	}
}

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", s.create)
	mux.HandleFunc("GET /tasks", s.list)
	mux.HandleFunc("GET /tasks/{id}", s.get)
	mux.HandleFunc("PUT /tasks/{id}", s.update)
	mux.HandleFunc("DELETE /tasks/{id}", s.remove)
	mux.HandleFunc("GET /healthz", s.health)
	return mux
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.logger.Warn("invalid task payload", "error", err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if in.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}

	s.mu.Lock()
	s.nextID++
	id := strconv.Itoa(s.nextID)
	t := Task{ID: id, Title: in.Title, CreatedAt: time.Now().UTC()}
	s.tasks[id] = t
	s.mu.Unlock()

	s.logger.Info("task created", "task_id", id, "title", t.Title)
	writeJSON(w, http.StatusCreated, t)
}

func (s *Service) list(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t)
	}
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	t, ok := s.tasks[id]
	s.mu.RUnlock()
	if !ok {
		s.logger.Warn("task not found", "task_id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Service) update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Title *string `json:"title"`
		Done  *bool   `json:"done"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.logger.Warn("invalid task payload", "error", err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	s.mu.Lock()
	t, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		s.logger.Warn("task not found", "task_id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if in.Title != nil {
		t.Title = *in.Title
	}
	if in.Done != nil {
		t.Done = *in.Done
	}
	s.tasks[id] = t
	s.mu.Unlock()

	s.logger.Info("task updated", "task_id", id)
	writeJSON(w, http.StatusOK, t)
}

func (s *Service) remove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	_, ok := s.tasks[id]
	if ok {
		delete(s.tasks, id)
	}
	s.mu.Unlock()
	if !ok {
		s.logger.Warn("task not found", "task_id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	s.logger.Info("task deleted", "task_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
