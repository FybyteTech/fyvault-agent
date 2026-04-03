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
)

var socketPath = flag.String("socket", "/var/run/fyvault/health.sock", "health socket path")

func main() {
	flag.Parse()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", *socketPath)
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
