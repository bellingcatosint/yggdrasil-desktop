// Package windowsservice discovers a local Yggdrasil Windows service without
// changing its configuration, permissions, or lifecycle.
package windowsservice

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bellingcatosint/yggdrasil-desktop/internal/adminapi"
	"github.com/hjson/hjson-go/v4"
)

var (
	ErrServiceNotFound = errors.New("Yggdrasil Windows service was not found")
	ErrUnsupportedPlatform = errors.New("Windows service discovery is only available on Windows")
	ErrNoStaticConfig = errors.New("Yggdrasil service does not reference a static configuration file")
)

// State is a platform-neutral service state label.
type State string

const (
	StateUnknown State = "unknown"
	StateStopped State = "stopped"
	StateStartPending State = "start-pending"
	StateStopPending State = "stop-pending"
	StateRunning State = "running"
	StateContinuePending State = "continue-pending"
	StatePausePending State = "pause-pending"
	StatePaused State = "paused"
)

// ServiceInfo contains read-only facts obtained from SCM and the service config.
type ServiceInfo struct {
	Name string
	State State
	BinaryPath string
	Arguments []string
	ConfigPath string
	AdminListen string
}

// ParseServiceCommandLine parses the Windows SCM ImagePath command line using
// the standard backslash-before-quote rules used by Windows command-line APIs.
func ParseServiceCommandLine(commandLine string) ([]string, error) {
	var args []string
	for i := 0; i < len(commandLine); {
		for i < len(commandLine) && isSpace(commandLine[i]) { i++ }
		if i == len(commandLine) { break }
		var b strings.Builder
		quoted := false
		for i < len(commandLine) {
			if commandLine[i] == '\\' {
				start := i
				for i < len(commandLine) && commandLine[i] == '\\' { i++ }
				n := i-start
				if i < len(commandLine) && commandLine[i] == '"' {
					for j := 0; j < n/2; j++ { b.WriteByte('\\') }
					if n%2 == 1 { b.WriteByte('"'); i++; continue }
					if quoted && i+1 < len(commandLine) && commandLine[i+1] == '"' {
						b.WriteByte('"'); i += 2; continue
					}
					quoted = !quoted; i++; continue
				}
				for j := 0; j < n; j++ { b.WriteByte('\\') }
				continue
			}
			if commandLine[i] == '"' {
				if quoted && i+1 < len(commandLine) && commandLine[i+1] == '"' { b.WriteByte('"'); i += 2; continue }
				quoted = !quoted; i++; continue
			}
			if !quoted && isSpace(commandLine[i]) { break }
			b.WriteByte(commandLine[i]); i++
		}
		if quoted { return nil, errors.New("unterminated quote in service command line") }
		args = append(args, b.String())
		for i < len(commandLine) && isSpace(commandLine[i]) { i++ }
	}
	if len(args) == 0 { return nil, errors.New("empty service command line") }
	return args, nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

// ResolveConfigPath extracts the path supplied through -useconffile. The
// returned path is empty for -autoconf, stdin, or a service without that flag.
func ResolveConfigPath(args []string) (string, error) {
	for i := 1; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-useconffile=") {
			path := strings.TrimPrefix(a, "-useconffile=")
			if path == "" { return "", errors.New("-useconffile is missing its path") }
			return path, nil
		}
		if a == "-useconffile" {
			if i+1 >= len(args) || args[i+1] == "" { return "", errors.New("-useconffile is missing its path") }
			return args[i+1], nil
		}
		if a == "-useconf" || a == "-autoconf" { return "", nil }
	}
	return "", nil
}

// ParseAdminListen reads the root AdminListen setting from Yggdrasil's HJSON
// configuration and rejects non-loopback TCP endpoints before returning it.
func ParseAdminListen(data []byte) (string, error) {
	var config map[string]any
	if err := hjson.Unmarshal(data, &config); err != nil { return "", fmt.Errorf("parse Yggdrasil HJSON configuration: %w", err) }
	value, ok := config["AdminListen"]
	if !ok { return "", errors.New("configuration does not contain AdminListen") }
	endpoint, ok := value.(string)
	if !ok { return "", errors.New("AdminListen must be a string") }
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" { return "", errors.New("AdminListen is disabled") }
	if _, err := adminapi.New(endpoint); err != nil { return "", fmt.Errorf("unsafe or unsupported AdminListen endpoint: %w", err) }
	return endpoint, nil
}
