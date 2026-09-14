package orchestrator

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Codi-Devs/ventago-homelab/internal/config"
	"github.com/Codi-Devs/ventago-homelab/internal/workers"
)

type Server struct {
	cfg     config.Config
	workers []workers.Worker
	http    *http.Server
}

type healthBody struct {
	Status  string            `json:"status"`
	Model   string            `json:"ollama_model"`
	Workers map[string]string `json:"workers"`
}

func New(cfg config.Config) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:     cfg,
		workers: workers.Catalog(cfg),
	}
	mux.HandleFunc("GET /health", s.health)
	s.http = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	body := healthBody{
		Status:  "ok",
		Model:   s.cfg.OllamaModel,
		Workers: map[string]string{},
	}
	for _, wrk := range s.workers {
		body.Workers[wrk.Name()] = workers.StatusLabel(wrk, s.cfg.WorkerEnabled(wrk.Name()))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.HTTPAddr)
	if err != nil {
		return err
	}
	log.Printf("orchestrator listening on %s model=%s", s.cfg.HTTPAddr, s.cfg.OllamaModel)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.loop(ctx)
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.http.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.http.Shutdown(shutdownCtx)
		wg.Wait()
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			wg.Wait()
			return nil
		}
		return err
	}
}

func (s *Server) loop(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	s.dispatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatch(ctx)
		}
	}
}

func (s *Server) dispatch(ctx context.Context) {
	for _, wrk := range s.workers {
		if !s.cfg.WorkerEnabled(wrk.Name()) {
			continue
		}
		if err := wrk.Run(ctx); err != nil {
			log.Printf("worker %s: %v", wrk.Name(), err)
		}
	}
}
