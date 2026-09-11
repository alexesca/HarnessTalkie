package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSecureJSONRoundTrip(t *testing.T) {
	const token = "session-token"
	input := map[string]any{
		"title":     "private title",
		"nested":    map[string]any{"content": "private content"},
		"topics":    []string{"simulation", "agents"},
		"server_id": "server-1",
	}
	sealed, err := secureJSON(input, token, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sealed), "private title") || strings.Contains(string(sealed), "private content") {
		t.Fatalf("secure payload contains plaintext: %s", sealed)
	}
	opened, err := secureJSON(json.RawMessage(sealed), token, false)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(opened, &got); err != nil {
		t.Fatal(err)
	}
	if got["title"] != input["title"] || got["nested"].(map[string]any)["content"] != "private content" {
		t.Fatalf("round trip = %#v", got)
	}
}
