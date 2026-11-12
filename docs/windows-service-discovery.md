# Windows service discovery

## Purpose

Discover an installed local Yggdrasil Windows service and derive its Admin API endpoint without changing service configuration, permissions, or lifecycle.

## Read-only access model

The implementation uses the Windows Service Control Manager (SCM) through golang.org/x/sys/windows:

- SCM handle: SC_MANAGER_CONNECT | SC_MANAGER_ENUMERATE_SERVICE.
- Service handle: SERVICE_QUERY_CONFIG | SERVICE_QUERY_STATUS.
- No SC_MANAGER_ALL_ACCESS or SERVICE_ALL_ACCESS.
- No service start, stop, restart, configuration update, or DACL modification.

The service is located by its service name (Yggdrasil, case-insensitive). If it is not visible to the caller, discovery returns a clear error; the application must not silently fall back to guessed executable or configuration paths.

## Configuration resolution

The service's ImagePath is parsed as a Windows command line. The parser handles quoted paths and backslash/quote escaping. The executable and arguments are retained. Environment variables in the service command line are expanded before the configuration file is read.

Supported configuration modes:

- `-useconffile PATH`
- `-useconffile=PATH`

`-autoconf` and `-useconf` do not identify a static configuration file. Discovery returns `ErrNoStaticConfig` with service metadata rather than inventing a configuration path or reading an unrelated file.

## Admin API endpoint resolution

The root HJSON `AdminListen` value takes precedence when present. It is validated by `adminapi.New`, which permits only TCP endpoints using `localhost` or a numeric loopback IP address.

When the key is absent, discovery uses the upstream Yggdrasil v0.5.15 Windows platform default, `tcp://localhost:9001`. This is not a blind port scan and does not change the daemon configuration. The diagnostic command must still connect to this loopback endpoint and successfully call the read-only `list` command before reporting that the Admin API is available.

An explicit empty `AdminListen` or `AdminListen: "none"` remains disabled and does not trigger fallback. Wildcard, LAN, and remote endpoints are rejected. Custom Yggdrasil builds may override the platform default at build time; if so, the default endpoint will fail the live API check and must be configured explicitly.

## Errors and boundaries

Discovery distinguishes a missing service, unsupported platform, missing static config, unreadable config, malformed HJSON, disabled Admin API, and unsafe endpoint. It never edits the service or config.

The upstream Admin API is unauthenticated. Loopback validation is not authentication and does not prevent other local processes from accessing the daemon's Admin API directly. The client must not expose, proxy, or bind this API to a LAN or public interface.

## Verification status

Portable unit tests cover command-line parsing, config-path resolution, HJSON parsing, fallback selection, and endpoint restrictions. CI can verify compilation and tests on Windows runners, but a real runtime check against the user's installed Windows service remains separate and must be performed only with the user's authorization.
