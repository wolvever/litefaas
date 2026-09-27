package main

import (
	"strings"
	"testing"
)

func TestRequireTLSPair(t *testing.T) {
	cases := []struct {
		cert, key string
		https     bool
		wantErr   bool
	}{
		{"", "", false, false},
		{"  ", "\t", false, false},
		{"cert.pem", "key.pem", true, false},
		{"cert.pem", "", false, true},
		{"", "key.pem", false, true},
		{"cert.pem", "  ", false, true},
	}
	for _, tc := range cases {
		https, err := requireTLSPair(tc.cert, tc.key)
		if (err != nil) != tc.wantErr {
			t.Fatalf("cert=%q key=%q err=%v wantErr=%v", tc.cert, tc.key, err, tc.wantErr)
		}
		if https != tc.https {
			t.Fatalf("cert=%q key=%q https=%v want %v", tc.cert, tc.key, https, tc.https)
		}
		if err != nil {
			msg := err.Error()
			if !strings.Contains(msg, "tls-cert") || !strings.Contains(msg, "tls-key") {
				t.Fatalf("error should mention both flags: %v", err)
			}
		}
	}
}
