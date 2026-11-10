# Technical Audit — Project Expanse × Yggdrasil Desktop

**Audit snapshot:** 2026-11-10  
**Baseline:** Yggdrasil v0.5.15  
**Status:** Static source/documentation review completed for the first discovery pass. No Windows runtime tests, installation tests, or executable prototype have been performed.

## 1. Executive findings

The project is technically feasible as a desktop control center for an already installed Yggdrasil daemon. The official Admin API is a JSON interface, and the current v0.5.15 source exposes enough read operations to build a useful local status and diagnostics view.

The main engineering constraint is not the UI. It is the privilege boundary around the Windows service and the unauthenticated local Admin API. The app must remain a normal-user process for ordinary viewing and must never expose the Admin API beyond loopback. Operations that change system service state or write protected configuration need a separate, narrowly scoped elevation path.

There are already desktop GUI projects. The opportunity is therefore not to claim that no GUI exists. It is to build a more dependable, modern and maintainable experience, with structured API integration, safe privilege handling, explicit capability detection and a better first-run journey.

**Recommended first vertical slice:** detect an existing Windows installation → connect to its local Admin API → show the actual node identity, peers and sessions → explain clearly whether the network is working. Keep configuration editing, service lifecycle controls, automatic installation and updates outside the first prototype until their privilege and recovery paths are tested.

## 2. Current upstream baseline

| Question | Evidence | Status | Risk / implication |
|---|---|---|---|
| What is the current stable release? | Official release page lists v0.5.15, published 2026-10-24. | **Confirmed** | Use v0.5.15 as the initial compatibility baseline; do not hard-code assumptions for all future versions. |
| What is required to build upstream? | v0.5.15 release notes require Go 1.26. | **Confirmed** | Build tooling must track upstream Go requirements. |
| Are Windows installers published? | v0.5.15 assets include x64, x86 and ARM64 MSI packages. | **Confirmed** | Windows distribution is available, but availability is not proof of a well-supported platform. |
| Is Windows a first-class upstream platform? | Official Windows installation guide says best-effort support and explicitly says the port has no current maintainer. | **Confirmed** | Treat Windows as a higher-risk integration target; test every supported Windows release and avoid depending on undocumented behavior. |
| Does the Windows installer register a service? | MSI build script installs an automatic-start service named Yggdrasil, running as LocalSystem. | **Confirmed from installer source** | Service control and configuration writes cross a privilege boundary. |
| Where is the service configuration? | MSI service arguments point to %ALLUSERSPROFILE%\\Yggdrasil\\yggdrasil.conf; the installer generates that file if absent. | **Confirmed from installer source** | Discover the actual service command line/config path; do not assume a single path across manual and packaged installs. |

## 3. Admin API: available integration surface

### 3.1 Protocol and transport

The Admin API accepts JSON requests and returns JSON responses. Requests are newline-delimited. The request contains a request command name, optional arguments, and optional keepalive. The response includes a status and, where applicable, response data or an error.

The default Windows endpoint is tcp://localhost:9001. The source uses a local TCP listener on Windows; Linux and macOS default to a UNIX-domain socket. The API can be disabled with AdminListen: none.

The v0.5.15 release notes specify a 1 MiB request limit and a 10-second read timeout. The client should implement bounded reads, deadlines, clear connection-state reporting and strict JSON decoding.

### 3.2 Commands in the v0.5.15 source

The current source registers these command families:

| Command | Purpose | MVP relevance |
|---|---|---|
| list | Enumerate commands supported by the running daemon | **Essential** — runtime capability discovery |
| getSelf | Local node address, key, subnet and build version | **Essential** — identity and health context |
| getPeers | Direct peer state, URI, address, key, direction, uptime, latency, cost, counters and errors | **Essential** — connection status |
| getSessions | Active traffic sessions and byte counters | **Essential** — distinguish peer connectivity from recent traffic |
| getTree | Tree entries known to this node | Later — local-observer network view |
| getPaths | Established paths known through this node | Later — diagnostics and path visualization |
| addPeer / removePeer | Add or remove a peer at runtime | Later — state-changing controls |
| getTun | TUN adapter enabled state, name and MTU | Useful for platform diagnostics |
| getMulticastInterfaces | Multicast interface state | Useful for first-run diagnostics |

**Important documentation discrepancy:** the current v0.5.15 handler registrations do not match the older public Admin API page in every detail. For example, the web page documents getDHT, while the v0.5.15 source registers getTree and getPaths. The API's list response must therefore be treated as authoritative for the running daemon. Do not build the UI around a hard-coded command list copied from the website.

### 3.3 Runtime changes are not configuration persistence

The addPeer and removePeer handlers call the core link manager. They change the running node's peer links; they do not edit the on-disk configuration file. The word “persistent” in the core link type refers to maintaining/reconnecting that peer link while the process runs, not to writing the peer into the configuration file.

This distinction must be visible in the UI:
- **Add for this run** — runtime action through Admin API.
- **Save across restarts** — configuration-file operation, requiring a separate safe write and service-restart workflow.

Do not label a runtime-only peer change as permanently saved.

### 3.4 What the data does — and does not — mean

- getPeers describes directly connected peers, not every node in the network.
- getSessions describes recent/open traffic sessions, not merely configured peers.
- getTree and getPaths expose the view known to this local node.
- None of these responses justifies presenting a complete, global or authoritative map of Yggdrasil.

The Explore view must communicate “visible from this node” and show uncertainty or missing data rather than implying global visibility.

## 4. Security and privilege model

### 4.1 Admin API is unauthenticated

The v0.5.15 Admin API implementation contains the explicit source comment “TODO: Add authentication”. The Windows default binds to localhost, which limits remote network exposure, but it does not authenticate individual local processes. The API includes state-changing operations such as addPeer and removePeer.

**Design rule:** connect only to the loopback endpoint discovered for the local service. Never change the endpoint to a LAN/public bind address, never offer remote Admin API access as a convenience feature, and never proxy this socket through a network-facing component.

### 4.2 Separate ordinary UI from elevated operations

The Windows MSI installs Yggdrasil as a LocalSystem service. The GUI should not run permanently elevated.

Recommended privilege split:
- Normal application process: inspect service metadata, connect to loopback Admin API, display state, validate input and prepare proposed changes.
- Elevated operation: only when the user explicitly requests a protected action such as starting/stopping the service or writing the protected configuration.
- If a helper process is needed, keep its command surface narrow and typed. Do not build a generic elevated shell or pass arbitrary command strings from the UI.
- Do not alter the Yggdrasil service security descriptor or grant broad service-control rights to interactive users as a shortcut.

The existing Yggdrasil GUI implementation demonstrates why this deserves attention: its Windows code invokes sc stop/start and changes the service DACL with sc sdset after elevation. That is a real implementation choice, not a requirement imposed by Yggdrasil. Expanse should avoid changing system-wide service permissions merely to simplify the interface.

### 4.3 Configuration contains secrets

The configuration model includes private-key material or a private-key path, peer URI parameters such as passwords and pinned public keys, and the optional group password. Treat the configuration file as sensitive.

Requirements:
- Never log the full configuration or private key.
- Do not include peer passwords or group passwords in diagnostics, crash reports or telemetry.
- Keep backups private and restrict their file permissions.
- Do not silently regenerate a node key; changing it changes the node identity/address.
- Before any write, create a recoverable backup and show the user exactly what will change.

### 4.4 Updates and installation integrity

The official MSI build script pins and verifies the Wintun driver archive checksum. This is useful evidence of an upstream packaging control, but it does not remove the need for Expanse to verify downloaded installer provenance, signatures and hashes before offering automated installation or updates.

Automatic installation and update are **not part of the first prototype**.

## 5. Windows installation and service integration

The official MSI:
- installs the Yggdrasil executable, yggdrasilctl.exe and Wintun DLL under Program Files;
- creates the configuration and log directory under %ALLUSERSPROFILE%\\Yggdrasil;
- installs a service named Yggdrasil;
- runs the service as LocalSystem and configures automatic startup;
- passes an explicit -useconffile path to the service executable.

A path detail needs deliberate handling. The Windows platform defaults in the Go source set DefaultConfigFile to C:\\Program Files\\Yggdrasil\\yggdrasil.conf, while the MSI service command line uses the ProgramData/ALLUSERSPROFILE configuration path. The CLI uses its platform default path when trying to discover AdminListen. On a standard MSI install it can therefore fall back to the default localhost endpoint if it cannot find the service configuration at the platform-default path. This is not proof that the service itself is misconfigured; it is a reason not to infer the active service configuration path from one hard-coded constant.

**Recommended discovery method:** inspect the installed service's executable path and arguments, then resolve the actual -useconffile and log paths. Keep a fallback for manually installed or custom service configurations and show “path unknown” rather than guessing.

## 6. Existing GUI and adjacent projects

### 6.1 EdwardBrok / Yggdrasil GUI

Repository: https://github.com/EdwardBrok/yggdrasil-network-gui

This is a genuine GUI for the standard Yggdrasil service, not merely a concept. Its README lists Windows and Linux support, node information, peer statistics, configuration editing, service restart/stop and public-peer import. The repository is licensed GPL-2.0. Its latest published release is v1.3.1 (marked pre-release, 2025-08-30); the latest commit visible in the repository history is dated 2025-08-31. The repository also has open user requests concerning proxy support, help/documentation, UI behavior and build automation.

Static source review found these implementation patterns:
- invokes yggdrasilctl and parses human-readable table output with regular expressions;
- rewrites the Peers configuration block using regular expressions;
- writes the Windows configuration file directly;
- controls the Windows service with sc commands;
- elevates a command that changes the Yggdrasil service DACL.

These observations do not prove that the program is unsafe or unusable. They identify concrete areas where Expanse can pursue a different engineering standard: structured JSON/API integration, safer configuration handling, least-privilege service control, clearer error states and maintainable cross-platform architecture.

### 6.2 JB-SelfCompany / Yggstack-GUI

Repository: https://github.com/JB-SelfCompany/yggstack-gui

This is a different product category. Its README describes a Wails + Go + React desktop client built around yggstack and gVisor netstack, with SOCKS5 proxy and port-forwarding features and no TUN/admin requirement. It runs a userspace Yggdrasil stack rather than simply managing an already installed system daemon.

It is useful as a UI/framework and userspace-networking reference. It is not a direct substitute for the initial Expanse goal of managing the standard Windows Yggdrasil service. Its current implementation and release maturity still require independent verification before borrowing architectural assumptions.

### 6.3 Official Android and iOS clients

The official mobile clients embed the Go implementation through mobile bindings and integrate with platform VPN APIs:
- Android uses Android VpnService, with the Yggdrasil core running inside the app.
- iOS uses Apple's NetworkExtension packet tunnel provider.
- The mobile configuration disables the Admin API and configures the TUN interface for the platform-specific tunnel.

These are valuable references for embedding the core and handling platform tunnels, but they are not desktop service managers. For the first Windows desktop milestone, controlling the existing daemon through its local Admin API is the smaller and more compatible integration surface.

## 7. Preliminary product and architecture direction

**Keep the first prototype read-only.** Its job is to prove reliable detection, connection and interpretation before it is allowed to modify a user's network.

Suggested first vertical slice:
1. Detect the installed Yggdrasil service and its executable/configuration arguments.
2. Connect only to the local Admin API endpoint.
3. Call list and detect supported commands.
4. Read getSelf, getPeers, getSessions and, if available, getTun.
5. Show node identity, daemon version, peer state and recent traffic.
6. Distinguish “service stopped”, “Admin API unavailable”, “connected but no peers”, “peers connected”, and “traffic observed”.
7. Provide a clear route to the official installer if Yggdrasil is not installed; do not silently install it.

After that prototype is tested, decide whether to add runtime peer actions, configuration persistence and service lifecycle controls.

Wails with a Go backend and a modern web UI is a credible candidate because the application backend is already Go-oriented and Yggstack-GUI demonstrates this general stack. It is **not yet selected**. Compare it with a native Go UI and test Windows packaging, WebView2 availability, accessibility, startup time, update strategy and the security boundary between frontend and backend before locking the framework.

## 8. Open questions and verification plan

| Question | Current status | Next verification |
|---|---|---|
| Does the installed v0.5.15 Windows service bind Admin API only to loopback? | Source default says localhost; not runtime-tested. | Inspect listener binding on a clean Windows VM and confirm with socket inspection. |
| What exact command list and JSON schemas does the packaged daemon expose? | Static handlers reviewed; no live daemon queried. | Query list and every required read command on v0.5.15 x64. |
| Does runtime addPeer survive service restart? | Source indicates runtime link-manager operation, not config-file persistence. | Test add/remove, inspect config before/after, restart and verify. |
| Can a standard user read Admin API state? | Likely with default loopback endpoint; not runtime-tested. | Test under a non-admin Windows account. |
| What rights are required for service start/stop and protected config writes? | Service is LocalSystem; exact ACL behavior not tested. | Inspect service DACL and test on standard/admin accounts; use UAC only when required. |
| How should config path be discovered for custom installs? | MSI path is known; platform default can differ. | Parse service command line and test custom service arguments. |
| Does the current installer upgrade/uninstall cleanly across supported Windows versions? | Not tested. | VM tests for fresh install, upgrade, rollback and uninstall, including Wintun behavior. |
| Which UI framework is best? | Open. | Build a small read-only prototype in the leading candidates and compare real packaging/runtime behavior. |
| How does upstream want desktop integrations contributed? | Not yet discussed with maintainers. | After the first working prototype, prepare a concise upstream discussion rather than assuming acceptance. |

## 9. Decision

The audit supports proceeding with **Project Expanse × Yggdrasil Desktop**.

It does not support immediately building a full installer, configuration editor, service manager and network explorer at once. The sensible next move is a Windows read-only proof of concept using the official Admin API, followed by a controlled test of service discovery and privilege handling.

The product should differentiate itself through trustworthiness and usability, not through a claim of being the first GUI. The strongest early value is a clear, modern, accurate answer to one question: **is my Yggdrasil node actually working, and what can it currently see?**

## Sources

1. Yggdrasil v0.5.15 release: https://github.com/yggdrasil-network/yggdrasil-go/releases/tag/v0.5.15
2. Admin API documentation: https://yggdrasil-network.github.io/admin.html
3. Windows installation documentation: https://yggdrasil-network.github.io/installation-windows.html
4. Configuration reference: https://yggdrasil-network.github.io/configurationref.html
5. Yggdrasil Admin API implementation (v0.5.15): https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/admin.go
6. Peer and session API implementation (v0.5.15): https://github.com/yggdrasil-network/yggdrasil-go/tree/v0.5.15/src/admin
7. Core peer operations (v0.5.15): https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/core/api.go
8. Windows defaults (v0.5.15): https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/config/defaults_windows.go
9. Windows MSI build script (v0.5.15): https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/contrib/msi/build-msi.sh
10. EdwardBrok Yggdrasil GUI: https://github.com/EdwardBrok/yggdrasil-network-gui
11. Yggstack-GUI: https://github.com/JB-SelfCompany/yggstack-gui
12. Official Android client: https://github.com/yggdrasil-network/yggdrasil-android
13. Official iOS client: https://github.com/yggdrasil-network/yggdrasil-ios

---

This report is a static discovery snapshot, not a security certification or a runtime compatibility guarantee. Update it when the Windows VM tests or upstream discussions produce new evidence.
