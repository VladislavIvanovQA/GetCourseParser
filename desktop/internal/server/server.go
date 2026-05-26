package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/getcourse-downloader/getcourse-downloader/internal/bundle"
	"github.com/getcourse-downloader/getcourse-downloader/internal/config"
	"github.com/getcourse-downloader/getcourse-downloader/internal/download"
	"github.com/getcourse-downloader/getcourse-downloader/internal/ffmpeg"
	"github.com/getcourse-downloader/getcourse-downloader/internal/jobs"
)

type Server struct {
	cfg *config.Config
	mux *http.ServeMux
}

func New(cfg *config.Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/pair", s.handlePair)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/lesson", s.handleLesson)
	s.mux.HandleFunc("/api/job", s.handleJob)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	log.Printf("GetCourse Downloader listening on http://%s", addr)
	log.Printf("Downloads: %s", s.cfg.DownloadsPath())
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.cors(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
	}
	return srv.ListenAndServe()
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-GC-Token, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ff, platform, install := ffmpeg.StatusForHealth()
	resp := map[string]any{
		"ok":       true,
		"version":  "1.2.0",
		"port":     s.cfg.Port,
		"platform": platform,
		"ffmpeg":   ff,
		"parallel": map[string]int{
			"max_jobs":  s.cfg.MaxParallelJobs,
			"max_files": s.cfg.MaxParallelFiles,
		},
	}
	if install != nil {
		resp["ffmpeg_install"] = install
	}
	writeJSON(w, resp)
}

func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Token for extension setup (localhost only).
	writeJSON(w, map[string]any{
		"token": s.cfg.Token,
		"port":  s.cfg.Port,
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if !s.checkToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"max_parallel_jobs":  s.cfg.MaxParallelJobs,
			"max_parallel_files": s.cfg.MaxParallelFiles,
			"download_dir":       s.cfg.DownloadsPath(),
		})
	case http.MethodPost:
		var body struct {
			MaxParallelJobs  *int `json:"max_parallel_jobs"`
			MaxParallelFiles *int `json:"max_parallel_files"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.MaxParallelJobs != nil {
			v := *body.MaxParallelJobs
			if v < 1 {
				v = 1
			} else if v > 8 {
				v = 8
			}
			s.cfg.MaxParallelJobs = v
		}
		if body.MaxParallelFiles != nil {
			v := *body.MaxParallelFiles
			if v < 1 {
				v = 1
			} else if v > 16 {
				v = 16
			}
			s.cfg.MaxParallelFiles = v
		}
		if err := s.cfg.Save(); err != nil {
			http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Settings updated: parallel_jobs=%d parallel_files=%d", s.cfg.MaxParallelJobs, s.cfg.MaxParallelFiles)
		writeJSON(w, map[string]any{
			"ok":                 true,
			"max_parallel_jobs":  s.cfg.MaxParallelJobs,
			"max_parallel_files": s.cfg.MaxParallelFiles,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLesson(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload bundle.LessonPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Lesson download started (referer=%s, videos=%d, async=%v)", payload.Referer, len(payload.VideoURLs), payload.Async)
	if payload.Async {
		jobID := jobs.Default.Start(s.cfg, &payload)
		writeJSON(w, map[string]any{
			"ok":     true,
			"job_id": jobID,
			"state":  "running",
		})
		return
	}
	result, err := download.ProcessLesson(s.cfg, &payload)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	log.Printf("Done: %s (files=%d videos=%d)", result.Folder, result.FilesSaved, result.VideosSaved)
	writeJSON(w, result)
}

func (s *Server) handleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkToken(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	st := jobs.Default.Get(id)
	if st == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, st)
}

func (s *Server) checkToken(r *http.Request) bool {
	tok := r.Header.Get("X-GC-Token")
	if tok == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			tok = strings.TrimSpace(auth[7:])
		}
	}
	return tok == s.cfg.Token
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
