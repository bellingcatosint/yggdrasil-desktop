package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T, handler func(net.Conn, map[string]any)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req map[string]any
				if err := json.NewDecoder(c).Decode(&req); err != nil {
					return
				}
				handler(c, req)
			}(conn)
		}
	}()
	return "tcp://" + ln.Addr().String()
}

func writeJSON(t *testing.T, conn net.Conn, v any) {
	t.Helper()
	if err := json.NewEncoder(conn).Encode(v); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func TestNewRejectsNonLoopbackAndMalformedEndpoints(t *testing.T) {
	for _, endpoint := range []string{"tcp://192.0.2.1:9001", "tcp://example.com:9001", "http://127.0.0.1:9001", "tcp://127.0.0.1", "tcp://user@127.0.0.1:9001"} {
		if _, err := New(endpoint); err == nil {
			t.Errorf("New(%q) unexpectedly succeeded", endpoint)
		}
	}
	if _, err := New("tcp://192.0.2.1:9001"); !errors.Is(err, ErrNonLoopbackEndpoint) {
		t.Fatalf("expected loopback error, got %v", err)
	}
}

func TestGetSelfAcceptsMultilineResponse(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, req map[string]any) {
		if req["request"] != "getSelf" {
			t.Errorf("unexpected command: %v", req["request"])
		}
		_, _ = fmt.Fprint(c, "{\n  \"status\": \"success\",\n  \"response\": {\"build_version\": \"0.5.15\", \"address\": \"200::1\", \"routing_entries\": 7}\n}\n")
	})
	client, err := New(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.GetSelf(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.BuildVersion != "0.5.15" || got.Address != "200::1" || got.RoutingEntries != 7 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestEmptyPeerAndSessionArraysAreNonNil(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, req map[string]any) {
		var payload any
		switch req["request"] {
		case "getPeers":
			payload = map[string]any{"peers": []any{}}
		case "getSessions":
			payload = map[string]any{"sessions": []any{}}
		default:
			t.Errorf("unexpected command: %v", req["request"])
		}
		writeJSON(t, c, map[string]any{"status": "success", "response": payload})
	})
	client, _ := New(endpoint)
	peers, err := client.GetPeers(context.Background())
	if err != nil || peers == nil || len(peers) != 0 {
		t.Fatalf("peers=%v err=%v", peers, err)
	}
	sessions, err := client.GetSessions(context.Background())
	if err != nil || sessions == nil || len(sessions) != 0 {
		t.Fatalf("sessions=%v err=%v", sessions, err)
	}
}

func TestMissingArrayIsNotReportedAsEmpty(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, _ map[string]any) {
		writeJSON(t, c, map[string]any{"status": "success", "response": map[string]any{}})
	})
	client, _ := New(endpoint)
	if _, err := client.GetPeers(context.Background()); err == nil || !strings.Contains(err.Error(), "missing peers array") {
		t.Fatalf("expected missing peers array error, got %v", err)
	}
}

func TestAPIErrorEnvelope(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, _ map[string]any) {
		writeJSON(t, c, map[string]any{"status": "error", "error": "unknown action 'getself'"})
	})
	client, _ := New(endpoint)
	_, err := client.GetSelf(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "unknown action 'getself'" {
		t.Fatalf("expected APIError, got %v", err)
	}
}

func TestMalformedAndTrailingResponsesRejected(t *testing.T) {
	for name, response := range map[string]string{"malformed": "{not-json}\n", "trailing": "{\"status\":\"success\",\"response\":{}} {}\n"} {
		t.Run(name, func(t *testing.T) {
			endpoint := testServer(t, func(c net.Conn, _ map[string]any) { _, _ = fmt.Fprint(c, response) })
			client, _ := New(endpoint)
			_, err := client.GetSelf(context.Background())
			if err == nil {
				t.Fatal("expected response error")
			}
		})
	}
}

func TestUnknownRequiredCommand(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, _ map[string]any) {
		writeJSON(t, c, map[string]any{"status": "success", "response": map[string]any{"list": []any{
			map[string]any{"command": "getSelf"}, map[string]any{"command": "getPeers"},
		}}})
	})
	client, _ := New(endpoint)
	err := client.RequireReadOnlyCommands(context.Background())
	if err == nil || !strings.Contains(err.Error(), "getsessions") {
		t.Fatalf("expected missing getsessions error, got %v", err)
	}
}

func TestTimeout(t *testing.T) {
	endpoint := testServer(t, func(_ net.Conn, _ map[string]any) { time.Sleep(150 * time.Millisecond) })
	client, _ := New(endpoint)
	_, err := client.WithTimeout(20 * time.Millisecond).GetSelf(context.Background())
	if err == nil {
		t.Fatal("expected timeout")
	}
}

func TestResponseLimit(t *testing.T) {
	endpoint := testServer(t, func(c net.Conn, _ map[string]any) {
		_, _ = fmt.Fprint(c, "{\"status\":\"success\",\"response\":{\"build_name\":\"", strings.Repeat("x", 256), "\"}}\n")
	})
	client, _ := New(endpoint)
	_, err := client.WithMaxReplyBytes(64).GetSelf(context.Background())
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}
