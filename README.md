# Project Expanse × Yggdrasil Desktop

A desktop control center designed to make the Yggdrasil network easier to install, connect to, understand, and manage.

## Project status

**Initiated — discovery and technical audit.**

This repository is the project’s working home. No application code or platform integration is claimed to exist yet.

## Vision

Build a dependable desktop application around the existing Yggdrasil implementation. The application should lower the barrier to entry without replacing the protocol, forking the network, or duplicating existing tools unnecessarily.

The product is organized around three connected capabilities:

- **Connect** — install or locate Yggdrasil, configure a node, connect, and verify that the network is working.
- **Explore** — inspect the peers and network information visible to the local node, and discover services and projects.
- **Manage** — administer the local node and its connections with clear boundaries around privileged operations.

## First milestone

Deliver one complete, trustworthy user journey:

> Install → connect → see that the network is working.

The initial development target is Windows, followed by Linux and then macOS. Platform priorities may change if the technical audit identifies a safer or more practical path.

## Engineering principles

1. **Do not reimplement the network.** Use the official Yggdrasil implementation and supported interfaces.
2. **Audit before promising.** Verify the Admin API, configuration model, service management, installation and update paths, existing GUI projects, and upstream contribution options before fixing the implementation plan.
3. **Keep administration local.** Do not expose the local administrative interface to the network. Treat access to it as privileged.
4. **Make state observable.** Distinguish connected, disconnected, degraded, and unknown states. Never present partial peer visibility as a complete map of the network.
5. **Prefer upstream compatibility.** Avoid a protocol fork and keep integration maintainable against upstream releases.
6. **Document uncertainty.** Separate verified behavior from assumptions and platform-specific unknowns.
7. **Build incrementally.** Prove one end-to-end workflow before expanding into discovery and advanced node management.

## Initial work plan

1. Audit the current Yggdrasil release, documentation, Admin API, and supported configuration mechanisms.
2. Review existing desktop GUI projects and identify what they do and do not cover.
3. Verify Windows service installation, lifecycle management, permissions, and packaging constraints.
4. Define a minimal cross-platform architecture and threat model.
5. Prototype the first-run and connection-verification workflow.
6. Test recovery, failure states, and safe handling of administrative access.
7. Expand to Linux and macOS after the first workflow is reliable.

## Scope boundaries

The project does not initially aim to:

- replace or fork Yggdrasil;
- implement a new network protocol;
- expose the Admin API beyond the local machine;
- claim a complete or globally accurate network topology;
- promise automatic installation, updates, or privileged system changes before their safety and platform behavior are verified.

## Repository conventions

- Keep decisions and assumptions documented.
- Prefer small, reviewable changes.
- Record platform-specific behavior and test evidence.
- Do not mark a feature complete until its behavior has been tested on the target platform.

## License

No license has been selected yet. Until one is added, the repository contents are not granted an open-source license by default.
