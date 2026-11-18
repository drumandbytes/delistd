package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// healthcheckCmd is what the Docker HEALTHCHECK runs: the distroless image has
// no shell or curl, so the binary asks its own web server.
func healthcheckCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:    "healthcheck",
		Hidden: true, // for container health checks; not part of the stable CLI
		Short:  "Exit 0 if the local web UI answers",
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return checkHealth(cmd.Context(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "port the web UI listens on")
	return cmd
}

// checkHealth requests /healthz on addr and fails unless it answers 200.
func checkHealth(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("web UI not reachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("web UI answered %s", resp.Status)
	}
	return nil
}
