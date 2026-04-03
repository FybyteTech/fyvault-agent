//go:build linux

package privilege

import (
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// Drop removes elevated capabilities (CAP_NET_ADMIN, CAP_SYS_ADMIN) and
// sets no_new_privs to prevent future privilege escalation.
// Call this after the eBPF program is attached.
func Drop(logger *zap.Logger) error {
	// Drop CAP_NET_ADMIN and CAP_SYS_ADMIN - no longer needed after eBPF is attached.
	caps := []uintptr{
		12, // CAP_NET_ADMIN
		21, // CAP_SYS_ADMIN
	}

	for _, cap := range caps {
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, cap, 0, 0, 0); err != nil {
			logger.Warn("failed to drop capability",
				zap.Uint64("cap", uint64(cap)), zap.Error(err))
		}
	}

	// Prevent future privilege escalation.
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("failed to set no_new_privs: %w", err)
	}

	logger.Info("privileges dropped")
	return nil
}
