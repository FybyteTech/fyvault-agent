package health

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// Server exposes agent health over a Unix domain socket.
type Server struct {
	socketPath string
	startTime  time.Time
	keyring    *keyring.Keyring
	logger     *zap.Logger
	listener   net.Listener
}

// New creates a health server bound to the given Unix socket path.
func New(socketPath string, kr *keyring.Keyring, logger *zap.Logger) *Server {
	return &Server{
		socketPath: socketPath,
		startTime:  time.Now(),
		keyring:    kr,
		logger:     logger,
	}
}

// Start begins serving health status on the Unix socket.
func (s *Server) Start() error {
	// Remove stale socket file if present.
	os.Remove(s.socketPath)

	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0755); err != nil {
		return err
	}

	listener, err := net.Listen("unix", s.socketPath)
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

	s.logger.Info("health server listening", zap.String("socket", s.socketPath))
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

// Stop shuts down the health server and removes the socket file.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.socketPath)
}

func isClosedError(err error) bool {
	// net.ErrClosed is returned when Serve is called on a closed listener.
	return err != nil && err.Error() == "use of closed network connection"
}
