// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseConfig(t *testing.T) {
	for _, name := range []string{
		"CRSH_IPC_ENDPOINT",
		"CRSH_URL",
		"CRSH_IDLE_TIMEOUT",
		"CRSH_NUM_ATTR",
		"CRSH_ATTR_KEY_0",
		"CRSH_ATTR_VALUE_0",
		"CRSH_ATTR_KEY_1",
		"CRSH_ATTR_VALUE_1",
	} {
		t.Setenv(name, "")
	}

	tests := []struct {
		name    string
		setenv  map[string]string
		wantErr string
		want    *Config
	}{
		{
			name:    "missing URL",
			wantErr: "CRSH_URL not set",
		},
		{
			name: "invalid idle timeout",
			setenv: map[string]string{
				"CRSH_URL":          "test://example",
				"CRSH_IDLE_TIMEOUT": "invalid",
			},
			wantErr: "invalid CRSH_IDLE_TIMEOUT",
		},
		{
			name: "negative idle timeout",
			setenv: map[string]string{
				"CRSH_URL":          "test://example",
				"CRSH_IDLE_TIMEOUT": "-1",
			},
			wantErr: "invalid CRSH_IDLE_TIMEOUT: -1",
		},
		{
			name: "invalid attribute count",
			setenv: map[string]string{
				"CRSH_URL":      "test://example",
				"CRSH_NUM_ATTR": "invalid",
			},
			wantErr: "invalid CRSH_NUM_ATTR",
		},
		{
			name: "negative attribute count",
			setenv: map[string]string{
				"CRSH_URL":      "test://example",
				"CRSH_NUM_ATTR": "-1",
			},
			wantErr: "invalid CRSH_NUM_ATTR: -1",
		},
		{
			name: "valid configuration",
			setenv: map[string]string{
				"CRSH_IPC_ENDPOINT": "endpoint",
				"CRSH_URL":          "test://example",
				"CRSH_IDLE_TIMEOUT": "42",
				"CRSH_NUM_ATTR":     "2",
				"CRSH_ATTR_KEY_0":   "first",
				"CRSH_ATTR_VALUE_0": "one",
				"CRSH_ATTR_KEY_1":   "second",
				"CRSH_ATTR_VALUE_1": "two",
			},
			want: &Config{
				IPCEndpoint: "endpoint",
				URL:         "test://example",
				IdleTimeout: 42 * time.Second,
				Attributes: []Attribute{
					{Key: "first", Value: "one"},
					{Key: "second", Value: "two"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, value := range tt.setenv {
				t.Setenv(name, value)
			}

			got, err := ParseConfig(NewLogger(""))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ParseConfig error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseConfig() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
