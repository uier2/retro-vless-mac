package main

import (
	"fmt"
	"net/url"
	"strings"
)

func classify(s string) (string, error) {
	u, e := url.Parse(strings.TrimSpace(s))
	if e != nil {
		return "", fmt.Errorf("Invalid link")
	}
	if u.Scheme == "vless" {
		return "vless", nil
	}
	if u.Scheme == "https" && u.Hostname() != "" && u.User == nil {
		return "subscription", nil
	}
	return "", fmt.Errorf("Paste a vless:// connection or an https:// subscription URL")
}
