# Windows read-only diagnostic

This small command-line program checks the first end-to-end Expanse workflow on a Windows machine with Yggdrasil already installed.

It performs read-only operations only:

1. Finds the local Yggdrasil Windows service.
2. Reads its state, executable path, and configured file path.
3. Reads the local Admin API endpoint from that configuration.
4. Confirms that the endpoint is loopback-only.
5. Queries the daemon's advertised command list, node information, direct peers, and sessions.

It does **not** start or stop the service, edit configuration, change permissions, add or remove peers, or print public keys and peer connection URIs.

## Run on Windows

From the repository root, with Go installed:

```powershell
go run ./cmd/diagnose
```

Run it as the ordinary Windows user first; do not launch an elevated terminal for this check.

If the service is missing, stopped, inaccessible, or uses a configuration mode without a static file, the command reports the reason and stops rather than guessing paths or attempting system changes.

A successful result proves that service discovery and the read-only Admin API work together on that machine. It is not a full application or security certification.
