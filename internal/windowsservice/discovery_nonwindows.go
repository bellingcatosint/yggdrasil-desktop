//go:build !windows

package windowsservice

import "context"

// Discover is unavailable outside Windows; parsing helpers remain portable.
func Discover(context.Context) (ServiceInfo, error) { return ServiceInfo{}, ErrUnsupportedPlatform }
