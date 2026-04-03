package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"

	"github.com/fybyte/fyvault-agent/internal/config"
)

var healthAddr = flag.String("addr", config.DefaultHealthAddr(), "health endpoint address (socket path or host:port)")

func main() {
	flag.Parse()

	network := "unix"
	if runtime.GOOS == "windows" {
		network = "tcp"
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(network, *healthAddr)
			},
		},
	}

	resp, err := client.Get("http://localhost/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read response: %v\n", err)
		os.Exit(1)
	}

	var status map[string]interface{}
	if err := json.Unmarshal(body, &status); err != nil {
		fmt.Fprintf(os.Stderr, "invalid response: %v\n", err)
		os.Exit(1)
	}

	if s, ok := status["status"].(string); !ok || s != "ok" {
		fmt.Fprintf(os.Stderr, "unhealthy: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Println(string(body))
	os.Exit(0)
}
