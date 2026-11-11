package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bellingcatosint/yggdrasil-desktop/internal/adminapi"
	"github.com/bellingcatosint/yggdrasil-desktop/internal/windowsservice"
)

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "Yggdrasil diagnostic:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, out *os.File) error {
	service, err := windowsservice.Discover(ctx)
	if err != nil {
		if errors.Is(err, windowsservice.ErrNoStaticConfig) {
			return fmt.Errorf("service %q has no statically discoverable config path; automatic connection was not attempted", service.Name)
		}
		return fmt.Errorf("discover local Yggdrasil service: %w", err)
	}

	fmt.Fprintf(out, "Service: %s (%s)\n", service.Name, service.State)
	fmt.Fprintf(out, "Executable: %s\n", service.BinaryPath)
	fmt.Fprintf(out, "Configuration: %s\n", service.ConfigPath)
	fmt.Fprintf(out, "Admin API: %s\n", service.AdminListen)

	if service.State != windowsservice.StateRunning {
		return fmt.Errorf("service is not running; no Admin API connection was attempted")
	}

	client, err := adminapi.New(service.AdminListen)
	if err != nil {
		return fmt.Errorf("validate local Admin API endpoint: %w", err)
	}
	if err := client.RequireReadOnlyCommands(ctx); err != nil {
		return fmt.Errorf("check daemon read-only API commands: %w", err)
	}

	self, err := client.GetSelf(ctx)
	if err != nil {
		return fmt.Errorf("read local node information: %w", err)
	}
	peers, err := client.GetPeers(ctx)
	if err != nil {
		return fmt.Errorf("read direct peers: %w", err)
	}
	sessions, err := client.GetSessions(ctx)
	if err != nil {
		return fmt.Errorf("read traffic sessions: %w", err)
	}

	upPeers := 0
	for _, peer := range peers {
		if peer.Up {
			upPeers++
		}
	}

	fmt.Fprintf(out, "Daemon: %s %s\n", self.BuildName, self.BuildVersion)
	fmt.Fprintf(out, "Node address: %s\n", self.Address)
	fmt.Fprintf(out, "Routing entries: %d\n", self.RoutingEntries)
	fmt.Fprintf(out, "Direct peers: %d total, %d connected\n", len(peers), upPeers)
	fmt.Fprintf(out, "Active/recent sessions: %d\n", len(sessions))
	fmt.Fprintln(out, "Result: local read-only Admin API query succeeded.")
	return nil
}
