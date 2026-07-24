package main

import (
	"testing"
	"time"
)

func TestParseNormalizationInterval(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "default", want: 5 * time.Second},
		{name: "trimmed", value: " 250ms ", want: 250 * time.Millisecond},
		{name: "zero", value: "0s", wantErr: true},
		{name: "negative", value: "-1s", wantErr: true},
		{name: "invalid", value: "later", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseNormalizationInterval(test.value)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseNormalizationInterval(%q) returned no error", test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNormalizationInterval(%q): %v", test.value, err)
			}
			if got != test.want {
				t.Fatalf("parseNormalizationInterval(%q) = %s, want %s", test.value, got, test.want)
			}
		})
	}
}
