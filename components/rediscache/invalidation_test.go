package rediscache

import (
	"testing"
)

func TestInvalidationRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		keys []string
	}{
		{"single key", []string{"user:1"}},
		{"multi key", []string{"user:1", "user:2", "session:abc"}},
		{"empty slice", []string{}},
		{"nil slice", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := encodeInvalidation(tt.keys)
			if err != nil {
				t.Fatalf("encodeInvalidation: %v", err)
			}
			got, err := decodeInvalidation(payload)
			if err != nil {
				t.Fatalf("decodeInvalidation: %v", err)
			}
			if len(got) != len(tt.keys) {
				t.Fatalf("decoded %d keys, want %d (%v)", len(got), len(tt.keys), got)
			}
			for i := range tt.keys {
				if got[i] != tt.keys[i] {
					t.Errorf("key[%d] = %q, want %q", i, got[i], tt.keys[i])
				}
			}
		})
	}
}

func TestDecodeInvalidation_Malformed(t *testing.T) {
	if _, err := decodeInvalidation([]byte("not-json")); err == nil {
		t.Fatal("decodeInvalidation succeeded on malformed payload, want error")
	}
}

func TestNewPublisher(t *testing.T) {
	p := NewPublisher(nil, "invalidations")
	if p.channel != "invalidations" {
		t.Errorf("channel = %q, want %q", p.channel, "invalidations")
	}
}

func TestNewSubscriber(t *testing.T) {
	s := NewSubscriber(nil, "invalidations")
	if s.channel != "invalidations" {
		t.Errorf("channel = %q, want %q", s.channel, "invalidations")
	}
}
