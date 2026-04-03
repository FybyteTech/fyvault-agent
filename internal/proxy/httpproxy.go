package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/fybyte/fyvault-agent/internal/cloud"
	"github.com/fybyte/fyvault-agent/internal/keyring"
)

// HTTPProxyConfig holds the configuration for an HTTP forward proxy.
type HTTPProxyConfig struct {
	SecretName     string
	TargetHost     string
	TargetPort     int
	HeaderName     string
	HeaderTemplate string // e.g. "Bearer {{value}}"
	ProxyPort      int
}

// HTTPProxy intercepts outbound HTTP(S) connections and injects API key headers.
type HTTPProxy struct {
	config   HTTPProxyConfig
	keyring  *keyring.Keyring
	logger   *zap.Logger
	listener net.Listener
	server   *http.Server
}

// httpProxyInjectionConfig mirrors the JSON shape in BootSecret.InjectionConfig
// for API_KEY secrets.
type httpProxyInjectionConfig struct {
	TargetHost     string `json:"target_host"`
	TargetPort     int    `json:"target_port"`
	HeaderName     string `json:"header_name"`
	HeaderTemplate string `json:"header_template"`
	ProxyPort      int    `json:"proxy_port"`
}

// NewHTTPProxy creates an HTTP proxy from a boot secret.
func NewHTTPProxy(secret cloud.BootSecret, kr *keyring.Keyring, logger *zap.Logger) (*HTTPProxy, error) {
	var ic httpProxyInjectionConfig
	if err := json.Unmarshal(secret.InjectionConfig, &ic); err != nil {
		return nil, fmt.Errorf("failed to parse injection config for %s: %w", secret.Name, err)
	}

	cfg := HTTPProxyConfig{
		SecretName:     secret.Name,
		TargetHost:     ic.TargetHost,
		TargetPort:     ic.TargetPort,
		HeaderName:     ic.HeaderName,
		HeaderTemplate: ic.HeaderTemplate,
		ProxyPort:      ic.ProxyPort,
	}

	if cfg.TargetHost == "" {
		return nil, fmt.Errorf("target_host is required for HTTP proxy %s", secret.Name)
	}
	if cfg.TargetPort == 0 {
		cfg.TargetPort = 443
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Authorization"
	}
	if cfg.HeaderTemplate == "" {
		cfg.HeaderTemplate = "{{value}}"
	}

	return &HTTPProxy{config: cfg, keyring: kr, logger: logger}, nil
}

// Start begins listening for connections.
func (p *HTTPProxy) Start() error {
	var err error
	p.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.config.ProxyPort))
	if err != nil {
		return fmt.Errorf("HTTP proxy listen: %w", err)
	}

	// Update ProxyPort with the actual port in case 0 was specified.
	p.config.ProxyPort = p.listener.Addr().(*net.TCPAddr).Port

	p.server = &http.Server{Handler: p}
	go p.server.Serve(p.listener)
	return nil
}

// ServeHTTP handles CONNECT requests (for HTTPS) and regular HTTP requests.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Read secret from keyring.
	secret, err := p.keyring.Read(p.config.SecretName)
	if err != nil {
		p.logger.Error("secret unavailable", zap.String("name", p.config.SecretName), zap.Error(err))
		http.Error(w, "secret unavailable", http.StatusInternalServerError)
		return
	}

	// Build header value from template.
	headerValue := strings.Replace(p.config.HeaderTemplate, "{{value}}", string(secret), 1)

	// Clone request and inject header.
	outReq := r.Clone(r.Context())
	outReq.URL.Scheme = "https"
	outReq.URL.Host = fmt.Sprintf("%s:%d", p.config.TargetHost, p.config.TargetPort)
	outReq.Host = p.config.TargetHost
	outReq.Header.Set(p.config.HeaderName, headerValue)
	outReq.RequestURI = "" // must clear for http.Client

	// Forward request to the real target.
	resp, err := http.DefaultClient.Do(outReq)
	if err != nil {
		p.logger.Warn("upstream request failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response back to the client.
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *HTTPProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	// For CONNECT (HTTPS tunneling): simple passthrough for V1.
	// Header injection on CONNECT requires MITM TLS which is Phase 3 work.
	targetConn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		targetConn.Close()
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		targetConn.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional copy.
	go func() {
		io.Copy(targetConn, clientConn)
		targetConn.Close()
	}()
	io.Copy(clientConn, targetConn)
	clientConn.Close()
}

// Stop shuts down the proxy server.
func (p *HTTPProxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

// Name returns a human-readable name for this proxy instance.
func (p *HTTPProxy) Name() string {
	return fmt.Sprintf("http-proxy-%s", p.config.SecretName)
}

// ListenAddr returns the local address the proxy is listening on.
func (p *HTTPProxy) ListenAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", p.config.ProxyPort)
}

// Targets returns the eBPF redirect targets for this proxy.
func (p *HTTPProxy) Targets() []Target {
	return []Target{{
		DestHost:  p.config.TargetHost,
		DestPort:  uint16(p.config.TargetPort),
		ProxyPort: uint16(p.config.ProxyPort),
	}}
}
