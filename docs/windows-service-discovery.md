# Windows service discovery

## Purpose

Discover an installed local Yggdrasil Windows service and derive the Admin API endpoint from the service's actual configuration. The desktop application must not assume a fixed service executable path, config path, or default Admin API port.

## Read-only access model

The implementation uses the Windows Service Control Manager (SCM) through golang.org/x/sys/windows:

- SCM handle: SC_MANAGER_CONNECT | SC_MANAGER_ENUMERATE_SERVICE.
- Service handle: SERVICE_QUERY_CONFIG | SERVICE_QUERY_STATUS.
- No SC_MANAGER_ALL_ACCESS or SERVICE_ALL_ACCESS.
- No service start, stop, restart, configuration update, or DACL modification.

The service is located by its service name (Yggdrasil, case-insensitive). If it is not visible to the caller, discovery returns a clear error; the application must not silently fall back to guessed paths.

## Configuration resolution

The service's ImagePath is parsed as a Windows command line. The parser handles quoted paths and backslash/quote escaping. The executable and arguments are retained in the result.

Supported configuration modes:

- -useconffile PATH
- -useconffile=PATH

-autoconf and -useconf do not identify a static configuration file. Discovery returns ErrNoStaticConfig with the service metadata rather than inventing an endpoint.

For a static config, the package parses HJSON and reads the root AdminListen value. The endpoint is passed through the existing adminapi.New validator, which permits only localhost or numeric loopback TCP endpoints. Wildcard, LAN, and remote endpoints are rejected.

## Errors and boundaries

The discovery layer distinguishes a missing service, unsupported platform, missing static config, unreadable config, malformed HJSON, missing/disabled AdminListen, and unsafe endpoint. It never edits the service or config.

The Admin API itself remains unauthenticated upstream. Loopback validation is not authentication and does not prevent other local processes from accessing the daemon's Admin API directly.

## Verification status

Portable unit tests cover command-line parsing, config-path resolution, HJSON parsing, and endpoint restrictions. The Windows-specific SCM path still requires a real Windows runtime test, including a standard-user access check against an installed Yggdrasil MSI. Cross-compilation alone is not considered runtime verification.
