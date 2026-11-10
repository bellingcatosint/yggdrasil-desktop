# Read-only Admin API client — implementation note

**Implementation:** `internal/adminapi`  
**Upstream contract:** Yggdrasil v0.5.15  
**Status:** first Go client and unit tests implemented; no live-daemon or Windows runtime verification.

## Implemented

- `New` accepts only `tcp://localhost:port` or a numeric loopback IP endpoint; non-loopback hosts and malformed endpoints are rejected.
- The connected remote TCP address is checked again and must be loopback.
- Each call uses a bounded context/deadline and one connection per request.
- Requests include an explicit JSON `arguments` object and newline framing.
- Responses are decoded as JSON values, not single physical lines, so upstream pretty-printed responses are supported.
- Response size is bounded (4 MiB by default); requests are bounded to the upstream 1 MiB limit.
- API error envelopes are returned as typed `APIError` values.
- Runtime command discovery is supported through `List` and `RequireReadOnlyCommands`.
- Read methods implemented: `GetSelf`, `GetPeers`, `GetSessions`.
- Empty peer/session arrays remain valid empty arrays; missing arrays are treated as malformed payloads.
- No state-changing API method is exposed.

## Test coverage

The current unit tests cover:

1. Rejection of non-loopback and malformed endpoints.
2. Multiline/indented JSON response decoding.
3. Empty peer and session arrays.
4. Missing peer array detection.
5. Admin API error envelope handling.
6. Malformed JSON and trailing JSON values.
7. Missing required runtime command detection.
8. Read timeout.
9. Response-size limit.

Local verification performed against the committed source:

- `go test -race ./...` — passed.
- `go vet ./...` — passed.
- `GOOS=windows GOARCH=amd64 go test -c ./internal/adminapi` — Windows/amd64 test binary cross-compilation passed; this is not a Windows runtime test.
- Environment: Linux, Go 1.23.2.

These are local mock-server tests. They do not establish compatibility with an installed Yggdrasil daemon or Windows service.

## Known limitations / next checks

- Confirm the exact runtime JSON emitted by a packaged v0.5.15 daemon.
- Verify endpoint discovery from the actual Windows service configuration; endpoint construction is currently explicit.
- Add tests for context cancellation, connection refusal, missing/null response payloads, and malformed command-list entries.
- Review the response-size accounting under fragmented and slow network writes.
- Add Windows cross-compilation and eventually a real Windows VM integration test.
- Keep all API access local; this client must not be extended with remote endpoints as a convenience feature.

## Security note

Loopback-only validation is defense in depth, not authentication. Any local process able to reach the daemon's unauthenticated Admin API may still be able to invoke commands outside this client's read-only surface. Expanse's client does not change that upstream trust boundary.
