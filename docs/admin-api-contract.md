# Admin API Contract — Yggdrasil v0.5.15

**Status:** source-derived integration contract; not yet verified against a running Windows daemon.  
**Upstream baseline:** `yggdrasil-network/yggdrasil-go` tag `v0.5.15`.  
**Purpose:** define the smallest safe, read-only client surface for the first Expanse Desktop prototype.

## 1. Scope and safety boundary

The first prototype may call only these read operations:

- `list`
- `getSelf`
- `getPeers`
- `getSessions`

Other commands must be treated as unavailable to the MVP, even if the daemon advertises them. In particular, do not call `addPeer` or `removePeer`: they mutate the running node's peer links.

The Admin API is unauthenticated in this upstream version (the source contains `TODO: Add authentication`). The client must connect only to a locally discovered loopback endpoint. It must not accept a user-supplied remote host, bind or proxy the API, or expose it to the LAN.

This document is a source-derived contract, not a security certification or a guarantee that every packaged build has identical runtime behavior.

## 2. Transport and framing

The upstream handler uses a JSON decoder over a TCP connection and an encoder for responses.

- Send one JSON request object followed by a newline.
- Requests are JSON objects; the request name is in `request`.
- The optional `arguments` member contains a JSON object.
- `keepalive` is optional and defaults to false.
- With `keepalive: false`, the daemon sends one response and closes the connection.
- The server's decoder is limited to 1 MiB of input per connection.
- The server sets a 10-second read deadline for each decode operation.
- The server rejects unknown top-level request fields.
- The response encoder emits indented JSON followed by a newline. **Do not assume the response is a single physical line.** Decode one complete JSON value from the stream.

For upstream Yggdrasil v0.5.15 on Windows, the platform default is `tcp://localhost:9001`. A missing `AdminListen` key in a config file does not mean the API is disabled: Yggdrasil initializes the configuration with platform defaults before applying the file. The desktop diagnostic may use this documented Windows default only when the key is absent, and must verify it with a successful read-only `list` request. An explicit empty value or `AdminListen: "none"` disables the Admin API and must not fall back. Custom builds may override the compiled platform default.

## 3. Request and response envelope

### Request

```json
{
  "request": "getSelf",
  "arguments": {},
  "keepalive": false
}
```

The server initializes missing or JSON-null `arguments` to an empty object. The MVP should always send an explicit object.

### Response

```json
{
  "status": "success",
  "request": {
    "request": "getSelf",
    "arguments": {},
    "keepalive": false
  },
  "response": {
    "build_name": "example",
    "build_version": "0.5.15"
  }
}
```

For a failed request, `status` is `error` and `error` contains a human-readable message. `response` may be absent or null on failure. Treat status as authoritative; do not infer success merely because the TCP exchange completed.

The `response` member is command-specific JSON. Decode it only after checking `status == "success"`.

## 4. Runtime capability discovery

Call `list` first. Its command-specific response has this shape:

```json
{
  "list": [
    {
      "command": "getSelf",
      "description": "Show details about this node",
      "fields": []
    }
  ]
}
```

The list is generated from handlers registered by the running daemon. Command names are lowercased and sorted by the server. The application must tolerate unknown future commands and missing optional commands.

For the MVP, require `getSelf`, `getPeers`, and `getSessions`. If any is missing, report a compatibility limitation rather than attempting undocumented alternatives.

## 5. Read operation contracts

### 5.1 getSelf

Request arguments: `{}`.

Response payload:

| Field | JSON type | Meaning |
|---|---|---|
| `build_name` | string | Build name |
| `build_version` | string | Daemon build version |
| `key` | string | Node public key, hex encoded |
| `address` | string | Node's Yggdrasil IPv6 address |
| `routing_entries` | unsigned integer | Local routing-entry count |
| `subnet` | string | Node subnet |

Do not label `key` as a private key. The handler serializes the node public key.

### 5.2 getPeers

Request arguments: `{}` (optional `sort` may be `default`, `cost`, or `uptime`; the MVP can omit it).

Response payload:

```json
{
  "peers": [
    {
      "remote": "tcp://peer.example:1234",
      "up": true,
      "inbound": false,
      "address": "200::1",
      "key": "hex-public-key",
      "port": 1234,
      "priority": 1,
      "cost": 1,
      "bytes_recvd": 0,
      "bytes_sent": 0,
      "rate_recvd": 0,
      "rate_sent": 0,
      "uptime": 12.5,
      "latency": 1000000,
      "last_error_time": 0,
      "last_error": ""
    }
  ]
}
```

Field notes:

- `peers` is an array and may be empty.
- `remote` and `address` may be omitted.
- `up` is the peer link's current up/down state; it is not a global network-health verdict.
- `inbound` identifies the connection direction.
- `key` is the peer public key in hex.
- Byte and rate fields are serialized from the upstream `DataUnit` type, whose underlying type is an unsigned 64-bit integer.
- `uptime` is seconds as a floating-point number.
- `latency` and `last_error_time` are Go `time.Duration` values serialized as integer nanoseconds.
- `last_error` is optional and should be displayed carefully; do not include peer URIs with credentials in logs or telemetry.

The MVP must display direct peers only. It must not imply that this list is a complete network topology.

### 5.3 getSessions

Request arguments: `{}`.

Response payload:

```json
{
  "sessions": [
    {
      "address": "200::2",
      "key": "hex-public-key",
      "bytes_recvd": 0,
      "bytes_sent": 0,
      "uptime": 3.5
    }
  ]
}
```

- `sessions` is an array and may be empty.
- `uptime` is seconds as a floating-point number.
- Byte fields are unsigned integer values serialized from `DataUnit`.
- Sessions describe traffic sessions known to this node; they are not equivalent to configured peers.

## 6. Error and compatibility behavior

The client should distinguish at least:

1. Service not found.
2. Service stopped.
3. Service running but Admin API endpoint cannot be resolved.
4. TCP connection refused or timed out.
5. Invalid or incomplete JSON response.
6. Valid response with `status: error`.
7. Daemon responds but required command is not advertised.
8. Required command succeeds but returns malformed or unexpected payload.
9. Daemon is reachable, but peer list is empty.
10. One or more direct peers report `up: true`.

Do not collapse these into a generic “offline” state.

The client should set its own connection and read deadlines. It should cap response size, reject malformed JSON, and avoid logging raw request/response payloads by default. A future implementation must account for the fact that the upstream 1 MiB limit applies to the decoder's connection reader, not a documented per-message schema limit.

## 7. Explicitly out of scope for the first adapter

- `addPeer`, `removePeer`, or any other state-changing command.
- Config-file parsing or editing.
- Service start/stop/restart.
- Automatic installation or update.
- Remote Admin API endpoints.
- Global topology claims or network-wide discovery.
- Telemetry containing addresses, public keys, peer URIs, or traffic metadata.

## 8. Upstream source references

All source references are pinned to tag `v0.5.15`:

1. `src/admin/admin.go` — request/response envelopes, JSON decoding, framing, limits, deadlines, command registration, and unauthenticated API TODO.
2. `src/admin/getself.go` — getSelf payload.
3. `src/admin/getpeers.go` — getPeers request and peer payload.
4. `src/admin/getsessions.go` — getSessions payload.
5. `src/admin/gettree.go` and `src/admin/getpaths.go` — local tree/path data, intentionally excluded from the first adapter.

Pinned source URLs:

- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/admin.go
- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/getself.go
- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/getpeers.go
- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/getsessions.go
- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/gettree.go
- https://github.com/yggdrasil-network/yggdrasil-go/blob/v0.5.15/src/admin/getpaths.go
