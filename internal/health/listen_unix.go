//go:build !windows

package health

import (
	"net"
	"os"
	"path/filepath"
)

func platformListen(addr string) (net.Listener, error) {
	os.Remove(addr)
	if err := os.MkdirAll(filepath.Dir(addr), 0755); err != nil {
		return nil, err
	}
	return net.Listen("unix", addr)
}

func platformCleanup(addr string) {
	os.Remove(addr)
}
