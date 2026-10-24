package main

import (
	"strings"
	"testing"
)

func TestUnsupportedOptionsRejected(t *testing.T) {
	for _, q := range []string{"authority=required", "mode=multi", "ech=required", "pqv=required", "packetEncoding=packetaddr", "allowInsecure=true", "insecure=true", "ed=2048", "fp=imaginary", "sni=one&sni=two"} {
		s := strings.Split(sample, "#")[0] + "&" + q
		if _, e := parseNode(s); e == nil {
			t.Fatalf("accepted unsupported option %s", q)
		}
	}
}
