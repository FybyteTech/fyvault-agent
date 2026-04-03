package proxy

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// DBProxyConfig holds configuration for a database wire-protocol proxy.
type DBProxyConfig struct {
	SecretName string
	DBType     string // "postgresql" for now
	TargetHost string
	TargetPort int
	ProxyPort  int
	Username   string
	Database   string
}

// DBProxy listens locally (no auth) and forwards to the real PostgreSQL
// server with credentials injected from the keyring.
type DBProxy struct {
	config   DBProxyConfig
	keyring  *keyring.Keyring
	logger   *zap.Logger
	listener net.Listener
	done     chan struct{}
}

// dbProxyInjectionConfig mirrors the JSON shape in BootSecret.InjectionConfig
// for DB_CREDENTIAL secrets.
type dbProxyInjectionConfig struct {
	DBType     string `json:"db_type"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	ProxyPort  int    `json:"proxy_port"`
	Username   string `json:"username"`
	Database   string `json:"database"`
}

// NewDBProxy creates a database proxy from a boot secret.
func NewDBProxy(secret cloud.BootSecret, kr *keyring.Keyring, logger *zap.Logger) (*DBProxy, error) {
	var ic dbProxyInjectionConfig
	if err := json.Unmarshal(secret.InjectionConfig, &ic); err != nil {
		return nil, fmt.Errorf("failed to parse injection config for %s: %w", secret.Name, err)
	}

	cfg := DBProxyConfig{
		SecretName: secret.Name,
		DBType:     ic.DBType,
		TargetHost: ic.TargetHost,
		TargetPort: ic.TargetPort,
		ProxyPort:  ic.ProxyPort,
		Username:   ic.Username,
		Database:   ic.Database,
	}

	if cfg.DBType == "" {
		cfg.DBType = "postgresql"
	}
	if cfg.TargetHost == "" {
		return nil, fmt.Errorf("target_host is required for DB proxy %s", secret.Name)
	}
	if cfg.TargetPort == 0 {
		cfg.TargetPort = 5432
	}

	return &DBProxy{config: cfg, keyring: kr, logger: logger}, nil
}

// Start begins accepting client connections.
func (p *DBProxy) Start() error {
	var err error
	p.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.config.ProxyPort))
	if err != nil {
		return fmt.Errorf("DB proxy listen: %w", err)
	}

	// Update ProxyPort with the actual port in case 0 was specified.
	p.config.ProxyPort = p.listener.Addr().(*net.TCPAddr).Port

	p.done = make(chan struct{})
	go p.acceptLoop()
	return nil
}

// Stop terminates the listener and all active connections.
func (p *DBProxy) Stop() error {
	if p.done != nil {
		close(p.done)
	}
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

// Name returns a human-readable name for this proxy.
func (p *DBProxy) Name() string {
	return fmt.Sprintf("db-proxy-%s", p.config.SecretName)
}

// ListenAddr returns the local address the proxy is listening on.
func (p *DBProxy) ListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", p.config.ProxyPort)
}

func (p *DBProxy) acceptLoop() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.done:
				return
			default:
				p.logger.Warn("accept error", zap.Error(err))
			}
			continue
		}
		go p.handleConnection(conn)
	}
}

func (p *DBProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// 1. Read the PostgreSQL startup message from the client.
	_, err := readStartupMessage(clientConn)
	if err != nil {
		p.logger.Error("failed to read startup", zap.Error(err))
		return
	}

	// 2. Connect to real PostgreSQL.
	targetAddr := net.JoinHostPort(p.config.TargetHost, fmt.Sprintf("%d", p.config.TargetPort))
	targetConn, err := net.DialTimeout("tcp", targetAddr, pgDialTimeout)
	if err != nil {
		p.logger.Error("failed to connect to target", zap.Error(err))
		sendPGError(clientConn, "failed to connect to database")
		return
	}
	defer targetConn.Close()

	// 3. Read password from keyring.
	password, err := p.keyring.Read(p.config.SecretName)
	if err != nil {
		p.logger.Error("failed to read DB password from keyring", zap.Error(err))
		sendPGError(clientConn, "credential unavailable")
		return
	}

	// 4. Send startup message to target with configured credentials.
	if err := sendStartupMessage(targetConn, p.config.Username, p.config.Database); err != nil {
		p.logger.Error("failed to send startup to target", zap.Error(err))
		return
	}

	// 5. Handle authentication with the target server.
	if err := handlePGAuth(targetConn, p.config.Username, string(password)); err != nil {
		p.logger.Error("target auth failed", zap.Error(err))
		sendPGError(clientConn, "authentication failed with upstream database")
		return
	}

	// 6. Forward messages from target until ReadyForQuery, then relay to client.
	if err := forwardUntilReady(targetConn, clientConn); err != nil {
		p.logger.Error("failed to establish session", zap.Error(err))
		return
	}

	// 7. Bidirectional relay.
	errc := make(chan error, 2)
	go func() { _, err := io.Copy(targetConn, clientConn); errc <- err }()
	go func() { _, err := io.Copy(clientConn, targetConn); errc <- err }()
	<-errc
}

// ---------------------------------------------------------------------------
// PostgreSQL v3 wire protocol helpers
// ---------------------------------------------------------------------------

const pgDialTimeout = 10 * 1e9 // 10 seconds in nanoseconds (time.Duration)

// readStartupMessage reads a PostgreSQL startup message from the connection.
// Returns the raw bytes of the message (including the length prefix).
func readStartupMessage(conn net.Conn) ([]byte, error) {
	// First 4 bytes: message length (including self).
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, fmt.Errorf("read startup length: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint32(lenBuf))
	if msgLen < 8 || msgLen > 10000 {
		return nil, fmt.Errorf("invalid startup message length: %d", msgLen)
	}

	msg := make([]byte, msgLen)
	copy(msg[:4], lenBuf)
	if _, err := io.ReadFull(conn, msg[4:]); err != nil {
		return nil, fmt.Errorf("read startup body: %w", err)
	}

	return msg, nil
}

// sendStartupMessage builds and sends a PostgreSQL v3 startup message.
func sendStartupMessage(conn net.Conn, user, database string) error {
	// Build parameter pairs: user, database, terminated by \0.
	var params []byte
	params = append(params, []byte("user")...)
	params = append(params, 0)
	params = append(params, []byte(user)...)
	params = append(params, 0)
	if database != "" {
		params = append(params, []byte("database")...)
		params = append(params, 0)
		params = append(params, []byte(database)...)
		params = append(params, 0)
	}
	params = append(params, 0) // terminating null

	// Message = length (4) + protocol version (4) + params.
	msgLen := 4 + 4 + len(params)
	msg := make([]byte, msgLen)
	binary.BigEndian.PutUint32(msg[0:4], uint32(msgLen))
	binary.BigEndian.PutUint32(msg[4:8], 196608) // v3.0
	copy(msg[8:], params)

	_, err := conn.Write(msg)
	return err
}

// handlePGAuth reads the authentication request from the server and responds.
// Supports AuthenticationCleartextPassword (3) and AuthenticationMD5Password (5).
func handlePGAuth(conn net.Conn, user, password string) error {
	// Read message type (1 byte) + length (4 bytes).
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read auth header: %w", err)
	}

	msgType := header[0]
	msgLen := int(binary.BigEndian.Uint32(header[1:5]))

	if msgType != 'R' {
		return fmt.Errorf("expected authentication request (R), got %c", msgType)
	}

	body := make([]byte, msgLen-4)
	if _, err := io.ReadFull(conn, body); err != nil {
		return fmt.Errorf("read auth body: %w", err)
	}

	if len(body) < 4 {
		return fmt.Errorf("auth body too short")
	}

	authType := binary.BigEndian.Uint32(body[0:4])

	switch authType {
	case 0:
		// AuthenticationOk - no password needed.
		return nil

	case 3:
		// AuthenticationCleartextPassword.
		return sendPasswordMessage(conn, password)

	case 5:
		// AuthenticationMD5Password.
		if len(body) < 8 {
			return fmt.Errorf("MD5 auth missing salt")
		}
		salt := body[4:8]
		hashed := pgMD5Password(user, password, salt)
		return sendPasswordMessage(conn, hashed)

	default:
		return fmt.Errorf("unsupported auth type: %d", authType)
	}
}

// sendPasswordMessage sends a PasswordMessage ('p') to the server.
func sendPasswordMessage(conn net.Conn, password string) error {
	pw := []byte(password)
	// Message: 'p' + length (4) + password + \0
	msgLen := 4 + len(pw) + 1
	msg := make([]byte, 1+msgLen)
	msg[0] = 'p'
	binary.BigEndian.PutUint32(msg[1:5], uint32(msgLen))
	copy(msg[5:], pw)
	msg[len(msg)-1] = 0

	_, err := conn.Write(msg)
	return err
}

// pgMD5Password computes the PostgreSQL MD5 authentication hash:
// "md5" + md5(md5(password + user) + salt)
func pgMD5Password(user, password string, salt []byte) string {
	// Inner: md5(password + user)
	inner := md5.Sum([]byte(password + user))
	innerHex := fmt.Sprintf("%x", inner)

	// Outer: md5(innerHex + salt)
	h := md5.New()
	h.Write([]byte(innerHex))
	h.Write(salt)
	outerHex := fmt.Sprintf("%x", h.Sum(nil))

	return "md5" + outerHex
}

// forwardUntilReady reads messages from src and copies them to dst until
// a ReadyForQuery ('Z') message is received.
func forwardUntilReady(src, dst net.Conn) error {
	for {
		// Read message type (1 byte) + length (4 bytes).
		header := make([]byte, 5)
		if _, err := io.ReadFull(src, header); err != nil {
			return fmt.Errorf("read message header: %w", err)
		}

		msgType := header[0]
		msgLen := int(binary.BigEndian.Uint32(header[1:5]))

		body := make([]byte, msgLen-4)
		if msgLen > 4 {
			if _, err := io.ReadFull(src, body); err != nil {
				return fmt.Errorf("read message body: %w", err)
			}
		}

		// Check for ErrorResponse before forwarding.
		if msgType == 'E' {
			return fmt.Errorf("server error during startup: %s", extractPGErrorMessage(body))
		}

		// Forward the complete message to the client.
		if _, err := dst.Write(header); err != nil {
			return fmt.Errorf("write header to client: %w", err)
		}
		if len(body) > 0 {
			if _, err := dst.Write(body); err != nil {
				return fmt.Errorf("write body to client: %w", err)
			}
		}

		// ReadyForQuery means the session is established.
		if msgType == 'Z' {
			return nil
		}
	}
}

// sendPGError sends an ErrorResponse message to a PostgreSQL client.
func sendPGError(conn net.Conn, message string) {
	// ErrorResponse: 'E' + length + field-type 'M' + message + \0 + terminator \0
	msgBytes := []byte(message)
	// body: 'S' + "ERROR\0" + 'M' + message + \0 + \0
	var body []byte
	body = append(body, 'S')
	body = append(body, []byte("ERROR")...)
	body = append(body, 0)
	body = append(body, 'M')
	body = append(body, msgBytes...)
	body = append(body, 0)
	body = append(body, 0) // terminator

	msgLen := 4 + len(body)
	msg := make([]byte, 1+msgLen)
	msg[0] = 'E'
	binary.BigEndian.PutUint32(msg[1:5], uint32(msgLen))
	copy(msg[5:], body)

	conn.Write(msg)
}

// extractPGErrorMessage extracts the 'M' (message) field from an ErrorResponse body.
func extractPGErrorMessage(body []byte) string {
	for i := 0; i < len(body); {
		fieldType := body[i]
		i++
		if fieldType == 0 {
			break
		}
		// Find the null terminator for this field value.
		end := i
		for end < len(body) && body[end] != 0 {
			end++
		}
		if fieldType == 'M' {
			return string(body[i:end])
		}
		i = end + 1
	}
	return "unknown error"
}
