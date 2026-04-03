//go:build !linux

package ebpf

import (
	"net"

	"go.uber.org/zap"
)

// Program is a no-op stub for non-Linux platforms.
type Program struct {
	logger *zap.Logger
}

// Load returns a stub Program on non-Linux platforms.
func Load(iface string, logger *zap.Logger) (*Program, error) {
	logger.Warn("eBPF not supported on this platform, running in stub mode")
	return &Program{logger: logger}, nil
}

// AddTarget is a no-op on non-Linux platforms.
func (p *Program) AddTarget(ip net.IP, dstPort, proxyPort uint16) error {
	p.logger.Debug("stub: AddTarget",
		zap.String("ip", ip.String()),
		zap.Uint16("port", dstPort),
		zap.Uint16("proxy", proxyPort))
	return nil
}

// RemoveTarget is a no-op on non-Linux platforms.
func (p *Program) RemoveTarget(ip net.IP, dstPort uint16) error {
	return nil
}

// Close is a no-op on non-Linux platforms.
func (p *Program) Close() error {
	return nil
}
