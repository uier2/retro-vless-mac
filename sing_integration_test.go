package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// This is an opt-in real-core test, with no external network or provider credentials.
// Run with SING_INTEGRATION=1 go test sing.go config.go sing_integration_test.go -v.
func TestSingRealVLESSLoopback(t *testing.T) {
	if os.Getenv("SING_INTEGRATION") != "1" {
		t.Skip("set SING_INTEGRATION=1 to run real Linux cores")
	}
	if runtime.GOOS != "linux" {
		t.Skip("Linux integration fixture")
	}
	sing := os.Getenv("SING_TEST_BINARY")
	xray := os.Getenv("XRAY_TEST_BINARY")
	for _, p := range []string{sing, xray} {
		if p == "" {
			t.Skip("set SING_TEST_BINARY and XRAY_TEST_BINARY to native test cores")
		}
		if _, err := os.Stat(p); err != nil {
			t.Skipf("test core unavailable: %v", err)
		}
	}
	const uuid = "11111111-1111-4111-8111-111111111111"
	marker := "real-vless-loopback-marker"
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, marker) }))
	defer target.Close()
	freePort := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	vp, sp := freePort(), freePort()
	dir := t.TempDir()
	writeJSON := func(name string, v interface{}) string {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(dir, name)
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		return p
	}
	server := map[string]interface{}{"log": map[string]interface{}{"loglevel": "warning"}, "inbounds": []interface{}{map[string]interface{}{"listen": "127.0.0.1", "port": vp, "protocol": "vless", "settings": map[string]interface{}{"decryption": "none", "clients": []interface{}{map[string]interface{}{"id": uuid}}}, "streamSettings": map[string]interface{}{"network": "tcp", "security": "none"}}}, "outbounds": []interface{}{map[string]interface{}{"protocol": "freedom"}}}
	xp := writeJSON("xray.json", server)
	start := func(binary string, args []string, port int) {
		cmd := exec.Command(binary, args...)
		logpath := filepath.Join(dir, filepath.Base(binary)+".log")
		f, e := os.Create(logpath)
		if e != nil {
			t.Fatal(e)
		}
		cmd.Stdout = f
		cmd.Stderr = f
		if e = cmd.Start(); e != nil {
			f.Close()
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		t.Cleanup(func() {
			cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			f.Close()
			if t.Failed() {
				b, _ := os.ReadFile(logpath)
				t.Logf("%s: %s", binary, b)
			}
		})
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case e := <-done:
				done <- e
				t.Fatalf("%s exited: %v", binary, e)
			default:
			}
			c, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
			if e == nil {
				c.Close()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatalf("%s did not listen", binary)
	}
	start(xray, []string{"run", "-config", xp}, vp)
	n, e := parseNode(fmt.Sprintf("vless://%s@example.com:%d?security=none&type=tcp", uuid, vp))
	if e != nil {
		t.Fatal(e)
	}
	b, e := singConfig(n, "127.0.0.1", "unused-tun")
	if e != nil {
		t.Fatal(e)
	}
	var cfg map[string]interface{}
	if e = json.Unmarshal(b, &cfg); e != nil {
		t.Fatal(e)
	}
	cfg["inbounds"] = []interface{}{map[string]interface{}{"type": "socks", "tag": "loopback-socks", "listen": "127.0.0.1", "listen_port": sp}}
	cp := writeJSON("sing.json", cfg)
	if out, e := exec.Command(sing, "check", "-c", cp).CombinedOutput(); e != nil {
		t.Fatalf("sing check: %v: %s", e, out)
	}
	start(sing, []string{"run", "-c", cp}, sp)
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{Timeout: 2 * time.Second}
		c, e := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", sp))
		if e != nil {
			return nil, e
		}
		fail := func(e error) (net.Conn, error) { c.Close(); return nil, e }
		c.SetDeadline(time.Now().Add(3 * time.Second))
		if _, e = c.Write([]byte{5, 1, 0}); e != nil {
			return fail(e)
		}
		reply := make([]byte, 2)
		if _, e = io.ReadFull(c, reply); e != nil {
			return fail(e)
		}
		if !bytes.Equal(reply, []byte{5, 0}) {
			return fail(fmt.Errorf("SOCKS greeting: %v", reply))
		}
		host, p, e := net.SplitHostPort(address)
		if e != nil {
			return fail(e)
		}
		ip := net.ParseIP(host).To4()
		if ip == nil {
			return fail(fmt.Errorf("fixture requires IPv4 target"))
		}
		port, _ := strconv.Atoi(p)
		request := append([]byte{5, 1, 0, 1}, ip...)
		request = binary.BigEndian.AppendUint16(request, uint16(port))
		if _, e = c.Write(request); e != nil {
			return fail(e)
		}
		head := make([]byte, 4)
		if _, e = io.ReadFull(c, head); e != nil {
			return fail(e)
		}
		if head[1] != 0 {
			return fail(fmt.Errorf("SOCKS CONNECT failed: %v", head))
		}
		length := 0
		switch head[3] {
		case 1:
			length = 4
		case 4:
			length = 16
		case 3:
			one := make([]byte, 1)
			if _, e = io.ReadFull(c, one); e != nil {
				return fail(e)
			}
			length = int(one[0])
		default:
			return fail(fmt.Errorf("bad SOCKS address: %v", head))
		}
		if _, e = io.CopyN(io.Discard, c, int64(length+2)); e != nil {
			return fail(e)
		}
		c.SetDeadline(time.Time{})
		return c, nil
	}
	transport := &http.Transport{DialContext: dial}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	res, e := client.Get(target.URL)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	body, e := io.ReadAll(res.Body)
	if e != nil {
		t.Fatal(e)
	}
	if res.StatusCode != 200 || string(body) != marker {
		t.Fatalf("unexpected HTTP result: %d %q", res.StatusCode, body)
	}
	t.Logf("verified HTTP marker %q through SOCKS -> sing-box VLESS -> Xray freedom", body)
}
