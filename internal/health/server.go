package health

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// Server exposes agent health over a platform-appropriate listener
// (Unix socket on Linux/macOS, TCP on Windows).
type Server struct {
	addr      string
	startTime time.Time
	keyring   *keyring.Keyring
	logger    *zap.Logger
	listener  net.Listener
}

// New creates a health server bound to the given address.
func New(addr string, kr *keyring.Keyring, logger *zap.Logger) *Server {
	return &Server{
		addr:      addr,
		startTime: time.Now(),
		keyring:   kr,
		logger:    logger,
	}
}

// Start begins serving health status.
func (s *Server) Start() error {
	listener, err := platformListen(s.addr)
	if err != nil {
		return err
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)

	go func() {
		if err := http.Serve(listener, mux); err != nil && !isClosedError(err) {
			s.logger.Error("health server error", zap.Error(err))
		}
	}()

	s.logger.Info("health server listening", zap.String("addr", s.addr))
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"uptime_seconds": int(time.Since(s.startTime).Seconds()),
		"secrets_count":  s.keyring.Count(),
	})
}

// Stop shuts down the health server and cleans up.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	platformCleanup(s.addr)
}

func isClosedError(err error) bool {
	// net.ErrClosed is returned when Serve is called on a closed listener.
	return err != nil && err.Error() == "use of closed network connection"
}
