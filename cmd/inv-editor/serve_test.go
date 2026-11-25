package main

import (
	"errors"
	"flag"
	"io"
	"testing"
)

func TestParseServeArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want serveOptions
	}{
		{"defaults", []string{"inv.yaml"},
			serveOptions{filePath: "inv.yaml", host: "127.0.0.1", port: 8080}},
		{"flags before file", []string{"--host", "0.0.0.0", "--port", "9090", "--readonly", "inv.yaml"},
			serveOptions{filePath: "inv.yaml", host: "0.0.0.0", port: 9090, readonly: true}},
		// Previously these flags were silently ignored, starting a writable
		// server on 127.0.0.1:8080.
		{"flags after file", []string{"inv.yaml", "--host", "0.0.0.0", "--port", "9090", "--readonly"},
			serveOptions{filePath: "inv.yaml", host: "0.0.0.0", port: 9090, readonly: true}},
		{"flags on both sides", []string{"--port=9090", "inv.yaml", "-readonly"},
			serveOptions{filePath: "inv.yaml", host: "127.0.0.1", port: 9090, readonly: true}},
		{"double dash ends flags", []string{"--readonly", "--", "--weird-name.yaml"},
			serveOptions{filePath: "--weird-name.yaml", host: "127.0.0.1", port: 8080, readonly: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServeArgs(tt.args, io.Discard)
			if err != nil {
				t.Fatalf("parseServeArgs(%q) error: %v", tt.args, err)
			}
			if *got != tt.want {
				t.Errorf("parseServeArgs(%q) = %+v, want %+v", tt.args, *got, tt.want)
			}
		})
	}
}

func TestParseServeArgsErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error // nil means any non-nil error
	}{
		{"no file", nil, errMissingInventoryFile},
		{"only flags", []string{"--readonly"}, errMissingInventoryFile},
		{"help", []string{"-h"}, flag.ErrHelp},
		{"help after file", []string{"inv.yaml", "--help"}, flag.ErrHelp},
		{"unknown flag", []string{"--nope", "inv.yaml"}, nil},
		{"unknown flag after file", []string{"inv.yaml", "--nope"}, nil},
		{"bad port after file", []string{"inv.yaml", "--port", "http"}, nil},
		{"two files", []string{"a.yaml", "b.yaml"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseServeArgs(tt.args, io.Discard)
			if err == nil {
				t.Fatalf("parseServeArgs(%q) succeeded, want error", tt.args)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("parseServeArgs(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}
