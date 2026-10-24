package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

const sample = "vless://11111111-1111-4111-8111-111111111111@example.com:443?security=tls&type=ws&path=%2Ftest&sni=example.com#My%20server"

func TestBuildConfig(t *testing.T) {
	n, e := parseNode(sample)
	if e != nil {
		t.Fatal(e)
	}
	if n.Name != "My server" {
		t.Fatal(n.Name)
	}
	b, e := makeConfig(n, 12345)
	if e != nil {
		t.Fatal(e)
	}
	var c map[string]interface{}
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), "127.0.0.1") || !strings.Contains(string(b), "/test") {
		t.Fatal(string(b))
	}
	for _, suffix := range []string{"security=none&type=xhttp", "security=banana", "security=tls&allowInsecure=1", "security=tls&type=grpc&flow=xtls-rprx-vision", "security=reality&pbk=short"} {
		if _, e := parseNode("vless://11111111-1111-4111-8111-111111111111@example.com:443?" + suffix); e == nil {
			t.Fatal("accepted", suffix)
		}
	}
	for _, s := range []string{"vless://bad@example.com:443", "vless://11111111-1111-4111-8111-111111111111@example.com:0", "vless://11111111-1111-4111-8111-111111111111@127.0.0.1:443"} {
		if _, e := parseNode(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
}
func TestPinnedEndpoint(t *testing.T) {
	n, e := parseNode(sample)
	if e != nil {
		t.Fatal(e)
	}
	b, e := makePinnedConfig(n, 12345, "203.0.113.4")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(b), `"address": "203.0.113.4"`) || !strings.Contains(string(b), `"udp": true`) {
		t.Fatal(string(b))
	}
}
func TestSubscription(t *testing.T) {
	for _, s := range []string{sample, base64.StdEncoding.EncodeToString([]byte(sample + "\nvmess://ignored")), base64.RawURLEncoding.EncodeToString([]byte(sample))} {
		ns, e := parseSubscription([]byte(s))
		if e != nil || len(ns) != 1 {
			t.Fatalf("%d %v", len(ns), e)
		}
	}
	if _, e := parseSubscription([]byte("<html>Login required</html>")); e == nil {
		t.Fatal("accepted HTML")
	}
}
