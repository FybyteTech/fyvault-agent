//go:build windows

package health

import "net"

func platformListen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func platformCleanup(_ string) {}
