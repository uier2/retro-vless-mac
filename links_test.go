package main

import "testing"

func TestClassify(t *testing.T) {
	for _, v := range []struct{ s, w string }{{"vless://abc@host:443", "vless"}, {"https://sub.example/connections/private", "subscription"}} {
		got, err := classify(v.s)
		if err != nil || got != v.w {
			t.Fatalf("got %q %v", got, err)
		}
	}
	for _, s := range []string{"http://example.com", "file:///etc/passwd", "", "vmess://abc", "https://user:pass@example.com/x"} {
		if _, err := classify(s); err == nil {
			t.Fatalf("accepted invalid input %q", s)
		}
	}
}
