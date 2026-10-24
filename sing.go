package main

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

func singConfig(n Node, endpoint, device string) ([]byte, error) {
	u, e := nodeParts(n.Link)
	if e != nil {
		return nil, e
	}
	q := u.Query()
	port, _ := strconv.Atoi(u.Port())
	out := map[string]interface{}{"type": "vless", "tag": "proxy", "server": endpoint, "server_port": port, "uuid": u.User.Username()}
	if f := q.Get("flow"); f != "" {
		out["flow"] = f
	}
	sec := q.Get("security")
	if sec == "tls" || sec == "reality" {
		name := q.Get("sni")
		if name == "" {
			name = u.Hostname()
		}
		tls := map[string]interface{}{"enabled": true, "server_name": name, "insecure": false}
		fp := q.Get("fp")
		if sec == "reality" && fp == "" {
			fp = "chrome"
		}
		if fp != "" {
			tls["utls"] = map[string]interface{}{"enabled": true, "fingerprint": fp}
		}
		if a := q.Get("alpn"); a != "" {
			tls["alpn"] = strings.Split(a, ",")
		}
		if sec == "reality" {
			tls["reality"] = map[string]interface{}{"enabled": true, "public_key": q.Get("pbk"), "short_id": q.Get("sid")}
		}
		out["tls"] = tls
	}
	switch q.Get("type") {
	case "ws":
		p := q.Get("path")
		if p == "" {
			p = "/"
		}
		tr := map[string]interface{}{"type": "ws", "path": p}
		if q.Get("host") != "" {
			tr["headers"] = map[string]string{"Host": q.Get("host")}
		}
		out["transport"] = tr
	case "grpc":
		out["transport"] = map[string]interface{}{"type": "grpc", "service_name": q.Get("serviceName")}
	}
	c := map[string]interface{}{"log": map[string]interface{}{"disabled": true}, "dns": map[string]interface{}{"servers": []interface{}{map[string]string{"address": "https://1.1.1.1/dns-query", "detour": "proxy"}}, "strategy": "prefer_ipv4"}, "inbounds": []interface{}{map[string]interface{}{"type": "tun", "tag": "tun-in", "interface_name": device, "inet4_address": "172.19.0.1/30", "inet6_address": "fdfe:dcba:9876::1/126", "mtu": 1500, "auto_route": true, "inet4_route_address": []string{"0.0.0.0/1", "128.0.0.0/1"}, "inet6_route_address": []string{"::/1", "8000::/1"}, "stack": "gvisor", "sniff": true}}, "outbounds": []interface{}{out, map[string]string{"type": "dns", "tag": "dns-out"}}, "route": map[string]interface{}{"auto_detect_interface": true, "final": "proxy", "rules": []interface{}{map[string]interface{}{"protocol": "dns", "outbound": "dns-out"}}}}
	return json.MarshalIndent(c, "", "  ")
}

// Keep net/url referenced explicitly in the wrapper build's supported stdlib surface.
var _ = url.QueryEscape
