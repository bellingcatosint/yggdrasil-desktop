// Package adminapi implements the read-only subset of the Yggdrasil Admin API
// used by the first Project Expanse desktop prototype.
package adminapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout  = 5 * time.Second
	defaultMaxReply = 4 << 20
	maxRequestBytes = 1 << 20
)

var (
	ErrNonLoopbackEndpoint = errors.New("admin API endpoint must use localhost or a loopback IP address")
	ErrRemotePeer          = errors.New("admin API connection did not terminate at a loopback address")
	ErrResponseTooLarge    = errors.New("admin API response exceeds configured limit")
)

// Client talks to one local Yggdrasil Admin API endpoint. It deliberately
// exposes no mutating commands.
type Client struct {
	endpoint string
	timeout  time.Duration
	maxReply int64
}

// New validates a tcp:// endpoint and rejects non-loopback hosts. Examples:
// tcp://127.0.0.1:9001 and tcp://localhost:9001.
func New(endpoint string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Admin API endpoint: %w", err)
	}
	if u.Scheme != "tcp" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("unsupported Admin API endpoint %q: expected tcp://loopback:port", endpoint)
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" || port == "" {
		return nil, fmt.Errorf("invalid Admin API endpoint %q: host and port are required", endpoint)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("invalid Admin API port %q", port)
	}
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsLoopback() {
			return nil, ErrNonLoopbackEndpoint
		}
	} else if !strings.EqualFold(host, "localhost") {
		return nil, ErrNonLoopbackEndpoint
	}
	return &Client{endpoint: net.JoinHostPort(host, port), timeout: defaultTimeout, maxReply: defaultMaxReply}, nil
}

// WithTimeout returns a copy with a different per-exchange timeout.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	clone := *c
	if timeout > 0 {
		clone.timeout = timeout
	}
	return &clone
}

// WithMaxReplyBytes returns a copy with a different response-size limit.
func (c *Client) WithMaxReplyBytes(limit int64) *Client {
	clone := *c
	if limit > 0 {
		clone.maxReply = limit
	}
	return &clone
}

type request struct {
	Request   string `json:"request"`
	Arguments any    `json:"arguments"`
	KeepAlive bool   `json:"keepalive,omitempty"`
}

type envelope struct {
	Status   string          `json:"status"`
	Error    string          `json:"error,omitempty"`
	Response json.RawMessage `json:"response"`
}

// APIError represents a valid Admin API response with status="error".
type APIError struct {
	Command string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Yggdrasil Admin API %s: %s", e.Command, e.Message)
}

func (c *Client) call(ctx context.Context, command string, args any, out any) error {
	if c == nil {
		return errors.New("nil Admin API client")
	}
	if command == "" {
		return errors.New("empty Admin API command")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.endpoint)
	if err != nil {
		return fmt.Errorf("connect to local Yggdrasil Admin API: %w", err)
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()

	// Defense in depth: even localhost must resolve to a loopback peer.
	remote, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok || remote.IP == nil || !remote.IP.IsLoopback() {
		return ErrRemotePeer
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set Admin API deadline: %w", err)
		}
	}

	payload, err := json.Marshal(request{Request: command, Arguments: args})
	if err != nil {
		return fmt.Errorf("encode Admin API request: %w", err)
	}
	if len(payload)+1 > maxRequestBytes {
		return fmt.Errorf("Admin API request exceeds %d bytes", maxRequestBytes)
	}
	payload = append(payload, '\n')
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("write Admin API request: %w", err)
	}

	limited := &io.LimitedReader{R: bufio.NewReader(conn), N: c.maxReply + 1}
	dec := json.NewDecoder(limited)
	var response envelope
	if err := dec.Decode(&response); err != nil {
		if limited.N <= 0 {
			return ErrResponseTooLarge
		}
		return fmt.Errorf("decode Admin API response: %w", err)
	}
	if c.maxReply+1-limited.N > c.maxReply {
		return ErrResponseTooLarge
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("Admin API returned multiple JSON values")
		}
		return fmt.Errorf("trailing data in Admin API response: %w", err)
	}
	if response.Status != "success" {
		if response.Status == "error" {
			return &APIError{Command: command, Message: response.Error}
		}
		return fmt.Errorf("unexpected Admin API status %q", response.Status)
	}
	if len(response.Response) == 0 || string(response.Response) == "null" {
		return errors.New("Admin API success response has no payload")
	}
	if err := json.Unmarshal(response.Response, out); err != nil {
		return fmt.Errorf("decode %s payload: %w", command, err)
	}
	return nil
}

// Command describes one command advertised by the running daemon.
type Command struct {
	Command     string   `json:"command"`
	Description string   `json:"description"`
	Fields      []string `json:"fields,omitempty"`
}
type ListResult struct {
	List []Command `json:"list"`
}

func (c *Client) List(ctx context.Context) (ListResult, error) {
	var result ListResult
	err := c.call(ctx, "list", struct{}{}, &result)
	return result, err
}

// RequireReadOnlyCommands verifies the minimum command set without invoking
// any command beyond list.
func (c *Client) RequireReadOnlyCommands(ctx context.Context) error {
	result, err := c.List(ctx)
	if err != nil {
		return err
	}
	available := make(map[string]bool, len(result.List))
	for _, item := range result.List {
		available[strings.ToLower(item.Command)] = true
	}
	for _, required := range []string{"getself", "getpeers", "getsessions"} {
		if !available[required] {
			return fmt.Errorf("running daemon does not advertise required command %q", required)
		}
	}
	return nil
}

type Self struct {
	BuildName      string `json:"build_name"`
	BuildVersion   string `json:"build_version"`
	Key            string `json:"key"`
	Address        string `json:"address"`
	RoutingEntries uint64 `json:"routing_entries"`
	Subnet         string `json:"subnet"`
}

func (c *Client) GetSelf(ctx context.Context) (Self, error) {
	var result Self
	err := c.call(ctx, "getSelf", struct{}{}, &result)
	return result, err
}

type Peer struct {
	Remote        string  `json:"remote,omitempty"`
	Up            bool    `json:"up"`
	Inbound       bool    `json:"inbound"`
	Address       string  `json:"address,omitempty"`
	Key           string  `json:"key"`
	Port          uint64  `json:"port"`
	Priority      uint64  `json:"priority"`
	Cost          uint64  `json:"cost"`
	BytesReceived uint64  `json:"bytes_recvd,omitempty"`
	BytesSent     uint64  `json:"bytes_sent,omitempty"`
	RateReceived  uint64  `json:"rate_recvd,omitempty"`
	RateSent      uint64  `json:"rate_sent,omitempty"`
	Uptime        float64 `json:"uptime,omitempty"`
	Latency       int64   `json:"latency,omitempty"`
	LastErrorTime int64   `json:"last_error_time,omitempty"`
	LastError     string  `json:"last_error,omitempty"`
}
type PeersResult struct {
	Peers *[]Peer `json:"peers"`
}

func (c *Client) GetPeers(ctx context.Context) ([]Peer, error) {
	var result PeersResult
	err := c.call(ctx, "getPeers", struct{}{}, &result)
	if err != nil {
		return nil, err
	}
	if result.Peers == nil {
		return nil, errors.New("getPeers payload is missing peers array")
	}
	return *result.Peers, nil
}

type Session struct {
	Address       string  `json:"address"`
	Key           string  `json:"key"`
	BytesReceived uint64  `json:"bytes_recvd"`
	BytesSent     uint64  `json:"bytes_sent"`
	Uptime        float64 `json:"uptime"`
}
type SessionsResult struct {
	Sessions *[]Session `json:"sessions"`
}

func (c *Client) GetSessions(ctx context.Context) ([]Session, error) {
	var result SessionsResult
	err := c.call(ctx, "getSessions", struct{}{}, &result)
	if err != nil {
		return nil, err
	}
	if result.Sessions == nil {
		return nil, errors.New("getSessions payload is missing sessions array")
	}
	return *result.Sessions, nil
}
