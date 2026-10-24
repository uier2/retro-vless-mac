package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingConfig(t *testing.T) {
	for _, q := range []string{"security=tls&type=ws&path=%2Fa&host=example.com", "security=tls&type=grpc&serviceName=test", "security=reality&type=tcp&flow=xtls-rprx-vision&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA&sid=abcd", "security=none&type=tcp"} {
		n, e := parseNode("vless://11111111-1111-4111-8111-111111111111@example.com:443?" + q)
		if e != nil {
			t.Fatal(e)
		}
		b, e := singConfig(n, "203.0.113.4", "utun25")
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(b), `"auto_route": true`) {
			t.Fatal("not TUN")
		}
		if core := os.Getenv("SING_TEST_BINARY"); core != "" {
			p := filepath.Join(t.TempDir(), "c.json")
			os.WriteFile(p, b, 0600)
			out, e := exec.Command(core, "check", "-c", p).CombinedOutput()
			if e != nil {
				t.Fatalf("%s: %v %s", q, e, out)
			}
		}
	}
}
