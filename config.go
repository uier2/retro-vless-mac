package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Node struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func nodeParts(s string) (*url.URL, error) {
	u, e := url.Parse(strings.TrimSpace(s))
	if e != nil || u.Scheme != "vless" || u.User == nil {
		return nil, fmt.Errorf("Invalid VLESS link")
	}
	if !uuidRE.MatchString(u.User.Username()) {
		return nil, fmt.Errorf("VLESS link needs a valid UUID")
	}
	if _, ok := u.User.Password(); ok {
		return nil, fmt.Errorf("Unexpected VLESS password")
	}
	p, e := strconv.Atoi(u.Port())
	if e != nil || p < 1 || p > 65535 || u.Hostname() == "" {
		return nil, fmt.Errorf("Invalid server address or port")
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || (ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast())) {
		return nil, fmt.Errorf("Local server addresses are not supported")
	}
	q, queryErr := url.ParseQuery(u.RawQuery)
	if queryErr != nil {
		return nil, fmt.Errorf("Invalid link options")
	}
	allowed := map[string]bool{"type": true, "security": true, "encryption": true, "flow": true, "sni": true, "fp": true, "alpn": true, "pbk": true, "sid": true, "path": true, "host": true, "serviceName": true, "mode": true, "headerType": true, "allowInsecure": true, "insecure": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 {
			return nil, fmt.Errorf("Unsupported or duplicate option: %s", key)
		}
	}
	if m := q.Get("mode"); m != "" && m != "gun" {
		return nil, fmt.Errorf("Unsupported gRPC mode")
	}
	for _, key := range []string{"allowInsecure", "insecure"} {
		if v := q.Get(key); v != "" && v != "0" && v != "false" {
			return nil, fmt.Errorf("Insecure TLS is not supported")
		}
	}
	if fp := q.Get("fp"); fp != "" {
		ok := false
		for _, known := range []string{"chrome", "firefox", "safari", "ios", "android", "edge", "360", "qq", "random", "randomized"} {
			if fp == known {
				ok = true
			}
		}
		if !ok {
			return nil, fmt.Errorf("Unsupported TLS fingerprint")
		}
	}
	typ := q.Get("type")
	if typ == "" {
		typ = "tcp"
	}
	if typ != "tcp" && typ != "ws" && typ != "grpc" {
		return nil, fmt.Errorf("Transport %q needs a different/newer client", typ)
	}
	sec := q.Get("security")
	if sec == "" {
		sec = "none"
	}
	if sec != "none" && sec != "tls" && sec != "reality" {
		return nil, fmt.Errorf("Unsupported security type")
	}
	if q.Get("allowInsecure") == "1" || q.Get("allowInsecure") == "true" || q.Get("insecure") == "1" {
		return nil, fmt.Errorf("Insecure TLS links are not supported")
	}
	if f := q.Get("flow"); f != "" && (f != "xtls-rprx-vision" || typ != "tcp" || sec == "none") {
		return nil, fmt.Errorf("Unsupported flow/transport combination")
	}
	if h := q.Get("headerType"); h != "" && h != "none" {
		return nil, fmt.Errorf("TCP header obfuscation is not supported")
	}
	if enc := q.Get("encryption"); enc != "" && enc != "none" {
		return nil, fmt.Errorf("This VLESS encryption requires a newer client")
	}
	if sec == "reality" {
		b, e := base64.RawURLEncoding.DecodeString(q.Get("pbk"))
		if e != nil || len(b) != 32 || q.Get("sni") == "" {
			return nil, fmt.Errorf("REALITY needs a server name and valid public key")
		}
		sid, e := hex.DecodeString(q.Get("sid"))
		if e != nil || len(sid) > 8 {
			return nil, fmt.Errorf("Invalid REALITY short ID")
		}
	}
	return u, nil
}
func parseNode(s string) (Node, error) {
	u, e := nodeParts(s)
	if e != nil {
		return Node{}, e
	}
	name := u.Fragment
	if name == "" {
		name = u.Hostname()
	}
	if len(name) > 160 {
		name = name[:160]
	}
	return Node{name, strings.TrimSpace(s)}, nil
}
func makeConfig(n Node, port int) ([]byte, error) {
	u, e := nodeParts(n.Link)
	if e != nil {
		return nil, e
	}
	q := u.Query()
	typ := q.Get("type")
	if typ == "" {
		typ = "tcp"
	}
	sec := q.Get("security")
	if sec == "" {
		sec = "none"
	}
	p, _ := strconv.Atoi(u.Port())
	stream := map[string]interface{}{"network": typ, "security": sec}
	if typ == "ws" {
		path := q.Get("path")
		if path == "" {
			path = "/"
		}
		ws := map[string]interface{}{"path": path}
		if q.Get("host") != "" {
			ws["headers"] = map[string]string{"Host": q.Get("host")}
		}
		stream["wsSettings"] = ws
	}
	if typ == "grpc" {
		stream["grpcSettings"] = map[string]interface{}{"serviceName": q.Get("serviceName"), "multiMode": q.Get("mode") == "multi"}
	}
	if sec == "tls" {
		tls := map[string]interface{}{"serverName": q.Get("sni"), "allowInsecure": false}
		if q.Get("fp") != "" {
			tls["fingerprint"] = q.Get("fp")
		}
		if q.Get("alpn") != "" {
			tls["alpn"] = strings.Split(q.Get("alpn"), ",")
		}
		stream["tlsSettings"] = tls
	}
	if sec == "reality" {
		fp := q.Get("fp")
		if fp == "" {
			fp = "chrome"
		}
		stream["realitySettings"] = map[string]interface{}{"serverName": q.Get("sni"), "fingerprint": fp, "publicKey": q.Get("pbk"), "shortId": q.Get("sid"), "spiderX": q.Get("spx")}
	}
	user := map[string]interface{}{"id": u.User.Username(), "encryption": "none"}
	if q.Get("flow") != "" {
		user["flow"] = q.Get("flow")
	}
	c := map[string]interface{}{"log": map[string]string{"loglevel": "none"}, "inbounds": []interface{}{map[string]interface{}{"listen": "127.0.0.1", "port": port, "protocol": "socks", "settings": map[string]interface{}{"auth": "noauth", "udp": true}}}, "outbounds": []interface{}{map[string]interface{}{"protocol": "vless", "settings": map[string]interface{}{"vnext": []interface{}{map[string]interface{}{"address": u.Hostname(), "port": p, "users": []interface{}{user}}}}, "streamSettings": stream}}}
	return json.MarshalIndent(c, "", "  ")
}
func makePinnedConfig(n Node, port int, endpoint string) ([]byte, error) {
	b, e := makeConfig(n, port)
	if e != nil {
		return nil, e
	}
	u, _ := url.Parse(n.Link)
	var c map[string]interface{}
	json.Unmarshal(b, &c)
	out := c["outbounds"].([]interface{})[0].(map[string]interface{})
	out["settings"].(map[string]interface{})["vnext"].([]interface{})[0].(map[string]interface{})["address"] = endpoint
	stream := out["streamSettings"].(map[string]interface{})
	if tls, ok := stream["tlsSettings"].(map[string]interface{}); ok && tls["serverName"] == "" {
		tls["serverName"] = u.Hostname()
	}
	return json.MarshalIndent(c, "", "  ")
}
func parseSubscription(b []byte) ([]Node, error) {
	s := strings.TrimSpace(string(b))
	if !strings.Contains(s, "vless://") {
		compact := strings.Join(strings.Fields(s), "")
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if d, e := enc.DecodeString(compact); e == nil {
				s = string(d)
				break
			}
		}
	}
	nodes := []Node{}
	seen := map[string]bool{}
	for _, line := range strings.Fields(s) {
		if strings.HasPrefix(line, "vless://") {
			if n, e := parseNode(line); e == nil && !seen[n.Link] {
				seen[n.Link] = true
				nodes = append(nodes, n)
			}
			if len(nodes) >= 500 {
				break
			}
		}
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("No compatible VLESS servers found. This build supports TCP, WebSocket and gRPC, with TLS or legacy REALITY. Ask the provider for a compatible VLESS link")
	}
	return nodes, nil
}
