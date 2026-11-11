//go:build windows

package windowsservice

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Discover reads the Yggdrasil service definition and its local configuration.
// It does not start/stop the service or alter service ACLs.
func Discover(ctx context.Context) (ServiceInfo, error) {
	if ctx == nil { ctx = context.Background() }
	if err := ctx.Err(); err != nil { return ServiceInfo{}, err }

	// Request only SCM connect + enumerate rights. mgr.Connect requests
	// SC_MANAGER_ALL_ACCESS, which is deliberately avoided here.
	hManager, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT|windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil { return ServiceInfo{}, fmt.Errorf("connect to Windows Service Control Manager for read-only discovery: %w", err) }
	m := &mgr.Mgr{Handle: hManager}
	defer m.Disconnect()

	names, err := m.ListServices()
	if err != nil { return ServiceInfo{}, fmt.Errorf("enumerate Windows services: %w", err) }
	var serviceName string
	for _, name := range names {
		if strings.EqualFold(name, "yggdrasil") { serviceName = name; break }
	}
	if serviceName == "" { return ServiceInfo{}, ErrServiceNotFound }
	if err := ctx.Err(); err != nil { return ServiceInfo{}, err }

	serviceNamePtr, err := windows.UTF16PtrFromString(serviceName)
	if err != nil { return ServiceInfo{}, fmt.Errorf("encode Yggdrasil service name: %w", err) }
	// Query-only service rights: no start, stop, change-config, or all-access rights.
	hService, err := windows.OpenService(m.Handle, serviceNamePtr, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if err != nil { return ServiceInfo{}, fmt.Errorf("open Yggdrasil service for query: %w", err) }
	s := &mgr.Service{Name: serviceName, Handle: hService}
	defer s.Close()

	cfg, err := s.Config()
	if err != nil { return ServiceInfo{}, fmt.Errorf("read Yggdrasil service configuration: %w", err) }
	status, err := s.Query()
	if err != nil { return ServiceInfo{}, fmt.Errorf("query Yggdrasil service state: %w", err) }
	args, err := ParseServiceCommandLine(cfg.BinaryPathName)
	if err != nil { return ServiceInfo{}, fmt.Errorf("parse Yggdrasil service ImagePath: %w", err) }
	configPath, err := ResolveConfigPath(args)
	if err != nil { return ServiceInfo{}, err }

	info := ServiceInfo{Name: serviceName, State: mapServiceState(status.State), BinaryPath: args[0], Arguments: append([]string(nil), args[1:]...), ConfigPath: configPath}
	if configPath == "" { return info, ErrNoStaticConfig }
	data, err := os.ReadFile(configPath)
	if err != nil { return info, fmt.Errorf("read Yggdrasil configuration %q: %w", configPath, err) }
	info.AdminListen, err = ParseAdminListen(data)
	if err != nil { return info, err }
	return info, nil
}

func mapServiceState(state svc.State) State {
	switch state {
	case svc.Stopped: return StateStopped
	case svc.StartPending: return StateStartPending
	case svc.StopPending: return StateStopPending
	case svc.Running: return StateRunning
	case svc.ContinuePending: return StateContinuePending
	case svc.PausePending: return StatePausePending
	case svc.Paused: return StatePaused
	default: return StateUnknown
	}
}
