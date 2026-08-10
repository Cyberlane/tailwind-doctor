package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
)

func frame(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload))
}

func TestServePublishesDiagnosticsForUnsavedChanges(t *testing.T) {
	root := t.TempDir()
	rootURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	documentURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(root, "page.tsx"))}).String()

	var input bytes.Buffer
	input.Write(frame(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": rootURI}}))
	input.Write(frame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
		"textDocument": map[string]any{"uri": documentURI, "text": `<div className="p-4 p-2" />`},
	}}))
	input.Write(frame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{
		"textDocument":   map[string]any{"uri": documentURI},
		"contentChanges": []map[string]any{{"text": `<div className="p-4" />`}},
	}}))
	input.Write(frame(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": nil}))
	input.Write(frame(t, map[string]any{"jsonrpc": "2.0", "method": "exit", "params": nil}))

	var output, log bytes.Buffer
	if err := Serve(&input, &output, &log); err != nil {
		t.Fatalf("Serve: %v (log: %s)", err, log.String())
	}
	reader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	messages := make([]map[string]any, 0)
	for {
		payload, err := readFrame(reader)
		if err != nil {
			break
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode output: %v", err)
		}
		messages = append(messages, decoded)
	}
	if len(messages) != 4 {
		t.Fatalf("messages = %#v", messages)
	}
	firstDiagnostics := messages[1]["params"].(map[string]any)["diagnostics"].([]any)
	if len(firstDiagnostics) != 1 || firstDiagnostics[0].(map[string]any)["code"] != "no-conflicting-utilities" {
		t.Fatalf("first diagnostics = %#v", firstDiagnostics)
	}
	secondDiagnostics := messages[2]["params"].(map[string]any)["diagnostics"].([]any)
	if len(secondDiagnostics) != 0 {
		t.Fatalf("second diagnostics = %#v", secondDiagnostics)
	}
}

func TestLSPPositionUsesUTF16Columns(t *testing.T) {
	position := lspPosition("😀 p-4", 1, len("😀 ")+1)
	if position["character"] != 3 {
		t.Fatalf("position = %+v, want character 3", position)
	}
}
