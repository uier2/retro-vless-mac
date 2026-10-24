package main

import "testing"

func TestSubscriptionRequestsVLESSList(t *testing.T) {
	r, e := subscriptionRequest("https://example.com/private-subscription")
	if e != nil {
		t.Fatal(e)
	}
	if got := r.Header.Get("User-Agent"); got != "v2rayN/6.0 RetroVLESS/1.0" {
		t.Fatalf("provider must receive VLESS-list-compatible user agent, got %q", got)
	}
}
