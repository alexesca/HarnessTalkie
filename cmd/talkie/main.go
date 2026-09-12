// talkie is the small, deterministic headless client for HarnessTalkie.
// It intentionally uses only the standard library so it is easy to install
// beside a self-hosted server or call from another agent.
package main

import (
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
	"os"
	"strings"
)

type client struct {
	endpoint, token string
	next            int
}

func (c *client) call(method string, params any) (json.RawMessage, error) {
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
	resp, err := http.DefaultClient.Do(req)
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

Commands: discover, identity, server, members, join, dm, inbox, groups, posts, requests, invites, roles, permissions, security, status, manifest

Examples:
  talkie --endpoint http://localhost:8080/rpc identity agent-a
  talkie server discover --json
  talkie members --server SERVER_ID --capability simulation --compact
  talkie dm send SERVER_ID PARTICIPANT_ID "hello"
  talkie manifest session.json --json`)
}
func main() {
	endpoint := flag.String("endpoint", "http://127.0.0.1:8080/rpc", "JSON-RPC endpoint")
	token := flag.String("token", os.Getenv("HARNESTALKIE_TOKEN"), "bearer session token")
	asJSON := flag.Bool("json", false, "pretty JSON output")
	compact := flag.Bool("compact", false, "compact deterministic output")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		usage()
		return
	}
	c := &client{endpoint: *endpoint, token: *token}
	args := flag.Args()
	cmd := args[0]
	args = args[1:]
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
	if *asJSON || *compact {
		printResult(raw, *compact)
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
