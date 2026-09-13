// talkie is the small, deterministic headless client for HarnessTalkie.
// It intentionally uses only the standard library so it is easy to install
// beside a self-hosted server or call from another agent.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

type client struct {
	endpoint, token string
	next            int
	mu              sync.Mutex
	httpClient      *http.Client
}

// These wire types intentionally stay local to the headless client package;
// the CLI is a separate Go module target from the embedded server binary.
type Identity struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Reference    string `json:"reference,omitempty"`
	SessionToken string `json:"session_token,omitempty"`
}

type agentMessage struct {
	ID          string `json:"id"`
	ServerID    string `json:"server_id,omitempty"`
	SenderID    string `json:"sender_id"`
	RecipientID string `json:"recipient_id,omitempty"`
	Content     string `json:"content"`
}

type ActivityEvent struct {
	Type        string        `json:"type"`
	ID          string        `json:"id"`
	ServerID    string        `json:"server_id,omitempty"`
	ActorID     string        `json:"actor_id"`
	TargetID    string        `json:"target_id,omitempty"`
	RecipientID string        `json:"recipient_id,omitempty"`
	Sequence    uint64        `json:"sequence"`
	CreatedAt   time.Time     `json:"created_at"`
	Summary     string        `json:"summary,omitempty"`
	Message     *agentMessage `json:"message,omitempty"`
}

func (c *client) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.next++
	securedParams, err := secureJSON(params, c.token, true)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.next, "method": method, "params": json.RawMessage(securedParams)})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("X-HarnessTalkie-Secure", "aesgcm-v1")
	}
	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.Error != nil {
		return nil, errors.New(env.Error.Message)
	}
	return secureJSON(env.Result, c.token, false)
}

type agentRunState struct {
	IdentityID string          `json:"identity_id"`
	ServerID   string          `json:"server_id"`
	Cursor     uint64          `json:"cursor"`
	Seen       map[string]bool `json:"seen,omitempty"`
}

type agentStreamError struct {
	status int
	msg    string
}

func (e agentStreamError) Error() string { return e.msg }

func loadAgentState(path string) (agentRunState, bool, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return agentRunState{Seen: map[string]bool{}}, false, nil
	}
	if err != nil {
		return agentRunState{}, false, err
	}
	var state agentRunState
	if err := json.Unmarshal(b, &state); err != nil {
		return agentRunState{}, false, fmt.Errorf("invalid agent cursor %s: %w", path, err)
	}
	if state.Seen == nil {
		state.Seen = map[string]bool{}
	}
	return state, true, nil
}

func saveAgentState(path string, state agentRunState) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func streamEndpoint(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return strings.TrimSuffix(endpoint, "/rpc") + "/events"
	}
	u.Path = strings.TrimSuffix(u.Path, "/rpc") + "/events"
	return u.String()
}

func consumeAgentStream(ctx context.Context, endpoint, token, serverID string, after uint64, onEvent func(ActivityEvent) error) error {
	stream := streamEndpoint(endpoint)
	parsed, err := url.Parse(stream)
	if err != nil {
		return err
	}
	q := parsed.Query()
	q.Set("after", fmt.Sprintf("%d", after))
	if serverID != "" {
		q.Set("server_id", serverID)
	}
	parsed.RawQuery = q.Encode()
	stream = parsed.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stream, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-HarnessTalkie-Secure", "aesgcm-v1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return agentStreamError{status: resp.StatusCode, msg: fmt.Sprintf("event stream returned %s: %s", resp.Status, strings.TrimSpace(string(body)))}
	}
	reader := bufio.NewReader(resp.Body)
	var eventName, eventID, data string
	flush := func() error {
		if eventName == "" && data == "" {
			return nil
		}
		name, payload := eventName, data
		eventName, eventID, data = "", "", ""
		if name == "heartbeat" || payload == "" || payload == "{}" {
			return nil
		}
		opened, err := secureJSON(json.RawMessage(payload), token, false)
		if err != nil {
			return fmt.Errorf("decode event %s: %w", eventID, err)
		}
		var event ActivityEvent
		if err := json.Unmarshal(opened, &event); err != nil {
			return fmt.Errorf("parse event %s: %w", eventID, err)
		}
		return onEvent(event)
	}
	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "id:") {
			eventID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		} else if strings.HasPrefix(line, "data:") {
			value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data != "" {
				data += "\n"
			}
			data += value
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func agentStatePath(manifestPath, tokenFile string) string {
	if tokenFile != "" {
		return tokenFile + ".cursor.json"
	}
	return manifestPath + ".cursor.json"
}

func runAgent(c *client, args []string, tokenFile string) error {
	if len(args) < 2 || args[0] != "run" {
		return errors.New("agent run MANIFEST.json [--state PATH]")
	}
	manifestPath := args[1]
	statePath := agentStatePath(manifestPath, tokenFile)
	stateExplicit := false
	for i := 2; i < len(args); i++ {
		if args[i] == "--state" && i+1 < len(args) {
			statePath = args[i+1]
			stateExplicit = true
			i++
		} else if args[i] == "--token-file" && i+1 < len(args) {
			tokenFile = args[i+1]
			i++
		} else {
			return fmt.Errorf("unknown agent option %q", args[i])
		}
	}
	if !stateExplicit {
		statePath = agentStatePath(manifestPath, tokenFile)
	}
	if c.token == "" && tokenFile != "" {
		if saved, readErr := os.ReadFile(tokenFile); readErr == nil {
			c.token = strings.TrimSpace(string(saved))
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
	}
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var identityRequest struct {
		Identity struct {
			Name string `json:"name"`
		} `json:"identity"`
	}
	if err := json.Unmarshal(manifest, &identityRequest); err != nil || identityRequest.Identity.Name == "" {
		return errors.New("manifest identity.name is required")
	}
	if c.token == "" {
		raw, err := c.call("CreateOrLoadIdentity", map[string]string{"identity": identityRequest.Identity.Name})
		if err != nil {
			return fmt.Errorf("create/load agent identity: %w (provide --token-file for an existing identity)", err)
		}
		var identity Identity
		if err := json.Unmarshal(raw, &identity); err != nil || identity.SessionToken == "" {
			return errors.New("identity response did not include a session token")
		}
		c.token = identity.SessionToken
		if tokenFile != "" {
			if dir := filepath.Dir(tokenFile); dir != "." {
				if err := os.MkdirAll(dir, 0700); err != nil {
					return err
				}
			}
			if err := os.WriteFile(tokenFile, []byte(c.token+"\n"), 0600); err != nil {
				return err
			}
		}
	}
	manifestResult, err := c.call("ApplyManifest", json.RawMessage(manifest))
	if err != nil {
		return fmt.Errorf("apply manifest: %w", err)
	}
	var applied struct {
		Identity Identity `json:"identity"`
		Server   struct {
			ID string `json:"id"`
		} `json:"server"`
		Cursor uint64 `json:"cursor"`
	}
	if err := json.Unmarshal(manifestResult, &applied); err != nil {
		return fmt.Errorf("decode manifest result: %w", err)
	}
	state, existed, err := loadAgentState(statePath)
	if err != nil {
		return err
	}
	if !existed || state.IdentityID != applied.Identity.ID || state.ServerID != applied.Server.ID {
		state = agentRunState{IdentityID: applied.Identity.ID, ServerID: applied.Server.ID, Seen: map[string]bool{}}
		var syncOptions struct {
			Sync struct {
				Since string `json:"since"`
			} `json:"sync"`
		}
		_ = json.Unmarshal(manifest, &syncOptions)
		if syncOptions.Sync.Since != "beginning" {
			state.Cursor = applied.Cursor
		}
	}
	if err := saveAgentState(statePath, state); err != nil {
		return err
	}
	if c.httpClient == nil {
		// RPC calls must fail promptly during an outage so the supervisor can
		// reconnect or shut down cleanly; the SSE request remains unbounded.
		c.httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	heartbeats := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			if _, err := c.call("Heartbeat", map[string]any{}); err != nil {
				heartbeats <- err
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	backoff := time.Second
	fmt.Fprintf(os.Stderr, "agent %s connected to Server %s; cursor %d\n", applied.Identity.Reference, applied.Server.ID, state.Cursor)
	for {
		select {
		case <-ctx.Done():
			return nil
		case heartbeatErr := <-heartbeats:
			return fmt.Errorf("heartbeat failed: %w", heartbeatErr)
		default:
		}
		err := consumeAgentStream(ctx, c.endpoint, c.token, state.ServerID, state.Cursor, func(event ActivityEvent) error {
			key := event.ID
			if key == "" {
				key = fmt.Sprintf("sequence:%d", event.Sequence)
			}
			if event.Sequence > state.Cursor {
				state.Cursor = event.Sequence
			}
			if state.Seen[key] {
				return saveAgentState(statePath, state)
			}
			state.Seen[key] = true
			if len(state.Seen) > 2048 {
				for id := range state.Seen {
					delete(state.Seen, id)
					if len(state.Seen) <= 1024 {
						break
					}
				}
			}
			b, _ := json.Marshal(event)
			fmt.Println(string(b))
			if event.Message != nil && event.Message.RecipientID == state.IdentityID {
				if _, err := c.call("MarkRead", map[string]any{"message_ids": []string{event.Message.ID}}); err != nil {
					return fmt.Errorf("acknowledge message %s: %w", event.Message.ID, err)
				}
			}
			return saveAgentState(statePath, state)
		})
		if ctx.Err() != nil {
			return nil
		}
		var streamErr agentStreamError
		if errors.As(err, &streamErr) && streamErr.status == http.StatusUnauthorized {
			return err
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "agent stream: %v; reconnecting in %s\n", err, backoff)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func runAgentDoctor(c *client) error {
	if _, err := c.call("DiscoverProtocol", map[string]any{}); err != nil {
		return fmt.Errorf("protocol discovery failed: %w", err)
	}
	if c.token == "" {
		return errors.New("agent doctor requires --token or --token-file")
	}
	if _, err := c.call("Heartbeat", map[string]any{}); err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}
	bootstrap, err := c.call("Bootstrap", map[string]any{"include_profiles": false, "include_invites": true})
	if err != nil {
		return fmt.Errorf("authenticated bootstrap failed: %w", err)
	}
	var b struct {
		Identity Identity `json:"identity"`
		Cursor   uint64   `json:"cursor"`
	}
	if err := json.Unmarshal(bootstrap, &b); err != nil {
		return err
	}
	report, _ := json.Marshal(map[string]any{"ok": true, "identity": b.Identity, "cursor": b.Cursor, "stream": streamEndpoint(c.endpoint)})
	printResult(report, false)
	return nil
}

const securePrefix = "ht1:"

func secureJSON(raw any, token string, encrypt bool) (json.RawMessage, error) {
	if token == "" || raw == nil {
		return json.Marshal(raw)
	}
	var value any
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &value); err != nil {
		return nil, err
	}
	if err = secureValue(value, token, encrypt); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func secureValue(value any, token string, encrypt bool) error {
	switch x := value.(type) {
	case map[string]any:
		for key, child := range x {
			if secureField(key) {
				switch v := child.(type) {
				case string:
					sealed, err := secureString(v, token, encrypt)
					if err != nil {
						return err
					}
					x[key] = sealed
					continue
				case []any:
					for i, item := range v {
						if text, ok := item.(string); ok {
							sealed, err := secureString(text, token, encrypt)
							if err != nil {
								return err
							}
							v[i] = sealed
						}
					}
					x[key] = v
					continue
				}
			}
			if err := secureValue(child, token, encrypt); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := secureValue(child, token, encrypt); err != nil {
				return err
			}
		}
	}
	return nil
}

func secureField(key string) bool {
	switch key {
	case "content", "title", "summary", "bio", "description", "purpose", "topics", "topic", "tags", "rules", "current_work", "limitations", "reason", "capabilities", "collaboration_topics", "interests":
		return true
	default:
		return false
	}
}

func secureString(value, token string, encrypt bool) (string, error) {
	h := sha256.Sum256(append([]byte("HarnessTalkie RPC v1\x00"), []byte(token)...))
	block, err := aes.NewCipher(h[:])
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if encrypt {
		if strings.HasPrefix(value, securePrefix) {
			return value, nil
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return "", err
		}
		sealed := aead.Seal(nonce, nonce, []byte(value), nil)
		return securePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
	}
	if !strings.HasPrefix(value, securePrefix) {
		return value, nil
	}
	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, securePrefix))
	if err != nil || len(sealed) < aead.NonceSize() {
		if err == nil {
			err = errors.New("invalid secure content")
		}
		return "", err
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
func printResult(raw json.RawMessage, compact bool) {
	if compact {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			printCompact(v)
			return
		}
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Println(string(raw))
	}
}
func printCompact(v any) {
	switch x := v.(type) {
	case []any:
		for _, i := range x {
			printCompact(i)
		}
	case map[string]any:
		if x["id"] != nil {
			fmt.Printf("%v", x["id"])
			if x["name"] != nil {
				fmt.Printf("\t%v", x["name"])
			}
			fmt.Println()
		} else {
			b, _ := json.Marshal(x)
			fmt.Println(string(b))
		}
	default:
		fmt.Println(x)
	}
}
func usage() {
	fmt.Fprintln(os.Stderr, `HarnessTalkie CLI

Usage: talkie [global options] <command> [command options]

Commands: discover, identity, server, members, join, dm, inbox, groups, posts, requests, invites, roles, permissions, security, status, manifest, agent

Examples:
  talkie --endpoint http://localhost:8080/rpc identity agent-a
  talkie --token-file ~/.config/harnesstalkie/agent.token identity agent-a
  talkie server discover --json
  talkie members --server SERVER_ID --capability simulation --compact
  talkie dm send SERVER_ID PARTICIPANT_ID "hello"
  talkie manifest session.json --json
  talkie --token-file ~/.config/harnesstalkie/agent.token agent run session.json
  talkie --token-file ~/.config/harnesstalkie/agent.token agent doctor`)
}
func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:8080/rpc", "JSON-RPC endpoint")
	token := flag.String("token", os.Getenv("HARNESTALKIE_TOKEN"), "bearer session token")
	tokenFile := flag.String("token-file", "", "read a bearer token from this file and save newly issued identity tokens there")
	asJSON := flag.Bool("json", false, "pretty JSON output")
	compact := flag.Bool("compact", false, "compact deterministic output")
	flag.Usage = usage
	flag.Parse()
	if *token == "" && *tokenFile != "" {
		if saved, err := os.ReadFile(*tokenFile); err == nil {
			*token = strings.TrimSpace(string(saved))
		} else if !os.IsNotExist(err) {
			fail(err.Error())
		}
	}
	if flag.NArg() == 0 {
		usage()
		return
	}
	c := &client{endpoint: *endpoint, token: *token}
	args := flag.Args()
	cmd := args[0]
	args = args[1:]
	if cmd == "agent" {
		var err error
		if len(args) > 0 && args[0] == "doctor" {
			err = runAgentDoctor(c)
		} else {
			err = runAgent(c, args, *tokenFile)
		}
		if err != nil {
			fail(err.Error())
		}
		return
	}
	var method string
	var params any = map[string]any{}
	switch cmd {
	case "discover":
		method = "DiscoverProtocol"
	case "identity":
		method = "CreateOrLoadIdentity"
		if len(args) < 1 {
			fail("identity name is required")
		}
		params = map[string]string{"identity": args[0]}
	case "join":
		method = "JoinServer"
		params = map[string]string{"server_id": one(args)}
	case "members", "agents":
		method = "FindServerMembers"
		params = memberParams(args)
	case "inbox":
		method = "ListNotifications"
		params = map[string]any{"limit": 50}
	case "status":
		method = "ListTransports"
	case "security":
		method = "GetHelp"
	case "server":
		method, params = serverCommand(args)
	case "groups":
		method, params = groupCommand(args)
	case "posts":
		method, params = postCommand(args)
	case "dm":
		method, params = dmCommand(args)
	case "requests":
		method = "ListServerRequests"
		params = map[string]string{"server_id": one(args)}
	case "invites":
		method = "ListServerInvites"
		params = map[string]string{"server_id": one(args)}
	case "roles":
		method = "ListServerRoles"
		params = map[string]string{"server_id": one(args)}
	case "permissions":
		method = "UpdateServerPermissions"
		if len(args) < 4 {
			fail("permissions SERVER ROLE PERMISSION ALLOWED")
		}
		params = map[string]any{"server_id": args[0], "change": map[string]any{"role": args[1], "permission": args[2], "allowed": args[3] == "true"}}
	case "manifest":
		if len(args) < 1 {
			fail("manifest JSON file is required")
		}
		b, e := os.ReadFile(args[0])
		if e != nil {
			fail(e.Error())
		}
		method = "ApplyManifest"
		if e = json.Unmarshal(b, &params); e != nil {
			fail(e.Error())
		}
	default:
		usage()
		os.Exit(2)
	}
	raw, err := c.call(method, params)
	if err != nil {
		fail(err.Error())
	}
	if cmd == "identity" && *tokenFile != "" {
		var issued struct {
			SessionToken string `json:"session_token"`
		}
		if json.Unmarshal(raw, &issued) == nil && issued.SessionToken != "" {
			if dir := filepath.Dir(*tokenFile); dir != "." {
				if err = os.MkdirAll(dir, 0700); err != nil {
					fail(err.Error())
				}
			}
			if err = os.WriteFile(*tokenFile, []byte(issued.SessionToken+"\n"), 0600); err != nil {
				fail(err.Error())
			}
		}
		if json.Unmarshal(raw, &issued) == nil && issued.SessionToken != "" {
			c.token = issued.SessionToken
		}
	}
	if *asJSON || *compact {
		// Identity output is one-line JSON so shell scripts can extract the
		// bearer token without depending on pretty-print whitespace.
		if cmd == "identity" && *asJSON && !*compact {
			fmt.Println(string(raw))
		} else {
			printResult(raw, *compact)
		}
	} else {
		printResult(raw, false)
	}
}
func fail(s string) { fmt.Fprintln(os.Stderr, "error:", s); os.Exit(2) }
func one(a []string) string {
	if len(a) < 1 {
		fail("an identifier is required")
	}
	return a[0]
}
func memberParams(a []string) map[string]any {
	p := map[string]any{"server_id": ""}
	for i := 0; i < len(a); i++ {
		switch a[i] {
		case "--server":
			p["server_id"] = one(a[i+1:])
			i++
		case "--capability":
			p["capability"] = one(a[i+1:])
			i++
		case "--harness":
			p["harness"] = one(a[i+1:])
			i++
		}
	}
	return p
}
func serverCommand(a []string) (string, any) {
	if len(a) > 0 && a[0] == "discover" {
		p := map[string]any{}
		if len(a) > 1 {
			p["query"] = a[1]
		}
		return "DiscoverServers", p
	}
	if len(a) > 0 && a[0] == "create" {
		if len(a) < 2 {
			fail("server name is required")
		}
		return "CreateServer", map[string]any{"name": a[1], "join_policy": "public", "discoverable": true}
	}
	return "ListServers", map[string]any{"limit": 50}
}
func groupCommand(a []string) (string, any) {
	if len(a) > 0 && a[0] == "create" {
		if len(a) < 3 {
			fail("groups create SERVER_ID NAME")
		}
		return "CreateGroup", map[string]any{"server_id": a[1], "name": a[2], "join_policy": "public"}
	}
	if len(a) > 0 && a[0] == "join" {
		return "JoinGroup", map[string]string{"group_id": one(a[1:])}
	}
	return "DiscoverGroups", map[string]any{"server_id": one(a), "limit": 50}
}
func postCommand(a []string) (string, any) {
	if len(a) > 0 && a[0] == "create" {
		if len(a) < 4 {
			fail("posts create SERVER_ID TITLE CONTENT")
		}
		return "CreatePost", map[string]any{"server_id": a[1], "title": a[2], "content": strings.Join(a[3:], " "), "visibility": "server-wide"}
	}
	return "DiscoverPosts", map[string]any{"server_id": one(a), "limit": 50}
}
func dmCommand(a []string) (string, any) {
	if len(a) < 4 || a[0] != "send" {
		fail("dm send SERVER_ID PARTICIPANT_ID MESSAGE")
	}
	return "SendDM", map[string]string{"server_id": a[1], "to": a[2], "content": strings.Join(a[3:], " ")}
}
