//go:build !linux

package privilege

import "go.uber.org/zap"

// Drop is a no-op on non-Linux platforms.
func Drop(logger *zap.Logger) error {
	logger.Warn("privilege drop not supported on this platform")
	return nil
}
