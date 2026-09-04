package main

import (
	"testing"
	"time"
)

func TestParseSyncPeriod(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 36000 * time.Second, false},
		{"600", 600 * time.Second, false},
		{"0", 0, true},
		{"-5", 0, true},
		{"ten", 0, true},
	}
	for _, tc := range cases {
		got, err := parseSyncPeriod(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("parseSyncPeriod(%q) = %v, %v; want %v, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestParseAllowSystemNamespaces(t *testing.T) {
	cases := []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{"", false, false},
		{"true", true, false},
		{"1", true, false},
		{"false", false, false},
		{"yes", false, true},
	}
	for _, tc := range cases {
		got, err := parseAllowSystemNamespaces(tc.in)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("parseAllowSystemNamespaces(%q) = %v, %v; want %v, err=%v", tc.in, got, err, tc.want, tc.wantErr)
		}
	}
}
