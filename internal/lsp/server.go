// Package lsp exposes Tailwind Doctor findings over the Language Server
// Protocol without adding a runtime dependency or changing analysis semantics.
package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/Cyberlane/tailwind-doctor/internal/audit"
)

const maximumMessageBytes = 16 << 20

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type document struct {
	URI     string
	Path    string
	Content string
}

type server struct {
	reader    *bufio.Reader
	writer    io.Writer
	log       io.Writer
	root      string
	analyzer  *audit.SourceAnalyzer
	documents map[string]document
	shutdown  bool
}

// Serve processes LSP messages until the client sends exit or closes stdin.
func Serve(input io.Reader, output, log io.Writer) error {
	current := &server{
		reader: bufio.NewReader(input), writer: output, log: log,
		documents: map[string]document{},
	}
	for {
		payload, err := readFrame(current.reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var incoming message
		if err := json.Unmarshal(payload, &incoming); err != nil {
			return fmt.Errorf("decode LSP message: %w", err)
		}
		stop, err := current.handle(incoming)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

func (server *server) handle(incoming message) (bool, error) {
	switch incoming.Method {
	case "initialize":
		return false, server.initialize(incoming)
	case "initialized":
		return false, nil
	case "textDocument/didOpen":
		return false, server.didOpen(incoming.Params)
	case "textDocument/didChange":
		return false, server.didChange(incoming.Params)
	case "textDocument/didSave":
		return false, server.didSave(incoming.Params)
	case "textDocument/didClose":
		return false, server.didClose(incoming.Params)
	case "shutdown":
		server.shutdown = true
		return false, server.write(response{JSONRPC: "2.0", ID: incoming.ID, Result: json.RawMessage("null")})
	case "exit":
		return true, nil
	case "workspace/didChangeWatchedFiles":
		return false, server.reload()
	case "$/cancelRequest", "workspace/didChangeConfiguration":
		return false, nil
	default:
		if len(incoming.ID) == 0 {
			return false, nil
		}
		return false, server.write(response{
			JSONRPC: "2.0", ID: incoming.ID,
			Error: &responseError{Code: -32601, Message: "method not found"},
		})
	}
}

func (server *server) reload() error {
	if server.root == "" {
		return nil
	}
	analyzer, err := audit.NewSourceAnalyzer(server.root)
	if err != nil {
		fmt.Fprintf(server.log, "tw-doctor lsp: reload: %v\n", err)
		return server.write(notification{JSONRPC: "2.0", Method: "window/showMessage", Params: map[string]any{
			"type": 1, "message": "Tailwind Doctor could not reload project context: " + err.Error(),
		}})
	}
	server.analyzer = analyzer
	uris := make([]string, 0, len(server.documents))
	for uri := range server.documents {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	for _, uri := range uris {
		if err := server.publish(server.documents[uri]); err != nil {
			return err
		}
	}
	return nil
}

func (server *server) initialize(incoming message) error {
	var params struct {
		RootURI          string `json:"rootUri"`
		RootPath         string `json:"rootPath"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
	}
	if err := json.Unmarshal(incoming.Params, &params); err != nil {
		return server.write(response{JSONRPC: "2.0", ID: incoming.ID,
			Error: &responseError{Code: -32602, Message: "invalid initialize parameters"}})
	}
	root := params.RootPath
	if params.RootURI != "" {
		root = filePathFromURI(params.RootURI)
	} else if len(params.WorkspaceFolders) > 0 {
		root = filePathFromURI(params.WorkspaceFolders[0].URI)
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	analyzer, err := audit.NewSourceAnalyzer(root)
	if err != nil {
		return server.write(response{JSONRPC: "2.0", ID: incoming.ID,
			Error: &responseError{Code: -32603, Message: err.Error()}})
	}
	server.root = filepath.Clean(root)
	server.analyzer = analyzer
	return server.write(response{JSONRPC: "2.0", ID: incoming.ID, Result: map[string]any{
		"capabilities": map[string]any{
			"textDocumentSync": map[string]any{"openClose": true, "change": 1, "save": map[string]any{"includeText": true}},
		},
		"serverInfo": map[string]any{"name": "tw-doctor", "version": audit.Version},
	}})
}

func (server *server) didOpen(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI  string `json:"uri"`
			Text string `json:"text"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	return server.update(params.TextDocument.URI, params.TextDocument.Text)
}

func (server *server) didChange(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	if err := json.Unmarshal(raw, &params); err != nil || len(params.ContentChanges) == 0 {
		return nil
	}
	return server.update(params.TextDocument.URI, params.ContentChanges[len(params.ContentChanges)-1].Text)
}

func (server *server) didSave(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Text *string `json:"text,omitempty"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	if params.Text != nil {
		return server.update(params.TextDocument.URI, *params.Text)
	}
	document, found := server.documents[params.TextDocument.URI]
	if !found {
		return nil
	}
	return server.publish(document)
}

func (server *server) didClose(raw json.RawMessage) error {
	var params struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil
	}
	delete(server.documents, params.TextDocument.URI)
	return server.write(notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: map[string]any{
		"uri": params.TextDocument.URI, "diagnostics": []any{},
	}})
}

func (server *server) update(uri, content string) error {
	if server.analyzer == nil || server.root == "" {
		return nil
	}
	path := filePathFromURI(uri)
	relative, err := filepath.Rel(server.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil
	}
	document := document{URI: uri, Path: filepath.ToSlash(relative), Content: content}
	server.documents[uri] = document
	return server.publish(document)
}

func (server *server) publish(document document) error {
	findings, err := server.analyzer.AnalyzeSource(document.Path, document.Content)
	if err != nil {
		fmt.Fprintf(server.log, "tw-doctor lsp: %v\n", err)
		findings = nil
	}
	diagnostics := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		diagnostics = append(diagnostics, map[string]any{
			"range": map[string]any{
				"start": lspPosition(document.Content, finding.Line, finding.Column),
				"end":   lspPosition(document.Content, finding.EndLine, finding.EndColumn),
			},
			"severity": diagnosticSeverity(finding),
			"code":     finding.Rule, "source": "tw-doctor", "message": finding.Message,
			"data": map[string]any{"confidence": finding.Confidence, "scored": finding.Scored},
		})
	}
	return server.write(notification{JSONRPC: "2.0", Method: "textDocument/publishDiagnostics", Params: map[string]any{
		"uri": document.URI, "diagnostics": diagnostics,
	}})
}

func diagnosticSeverity(finding audit.Finding) int {
	if finding.Scored {
		return 1
	}
	if finding.Severity == audit.SeverityWarn {
		return 2
	}
	return 3
}

func lspPosition(content string, line, byteColumn int) map[string]int {
	if line < 1 {
		line = 1
	}
	if byteColumn < 1 {
		byteColumn = 1
	}
	lines := strings.Split(content, "\n")
	if line > len(lines) {
		line = len(lines)
	}
	lineText := lines[line-1]
	byteOffset := byteColumn - 1
	if byteOffset > len(lineText) {
		byteOffset = len(lineText)
	}
	for byteOffset > 0 && byteOffset < len(lineText) && lineText[byteOffset]&0xc0 == 0x80 {
		byteOffset--
	}
	units := 0
	for _, character := range lineText[:byteOffset] {
		units += utf16.RuneLen(character)
	}
	return map[string]int{"line": line - 1, "character": units}
}

func filePathFromURI(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "file" {
		return ""
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return ""
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		path = "//" + parsed.Host + path
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

func (server *server) write(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(server.writer, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}

func readFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length: %w", err)
		}
		length = parsed
	}
	if length < 0 || length > maximumMessageBytes {
		return nil, fmt.Errorf("invalid LSP message length %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return bytes.Clone(payload), nil
}
