package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"digital.vasic.mcp/pkg/protocol"
)

// HTTPServerConfig holds configuration for the HTTP MCP server.
type HTTPServerConfig struct {
	// Address is the listen address (e.g., ":8080").
	Address string

	// ReadTimeout for the HTTP server.
	ReadTimeout time.Duration

	// WriteTimeout for the HTTP server.
	WriteTimeout time.Duration

	// MaxRequestSize is the max request body size in bytes.
	MaxRequestSize int64

	// HeartbeatInterval is the SSE heartbeat interval.
	HeartbeatInterval time.Duration
}

// DefaultHTTPServerConfig returns sensible defaults.
func DefaultHTTPServerConfig() HTTPServerConfig {
	return HTTPServerConfig{
		Address:           ":8080",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		MaxRequestSize:    10 * 1024 * 1024,
		HeartbeatInterval: 30 * time.Second,
	}
}

// sseClient represents a connected SSE client.
type sseClient struct {
	id      string
	writer  http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

// HTTPServer is an MCP server that communicates via HTTP with SSE.
type HTTPServer struct {
	info      protocol.ServerInfo
	config    HTTPServerConfig
	tools     map[string]*toolEntry
	resources map[string]*resourceEntry
	prompts   map[string]*promptEntry
	server    *http.Server
	mux       *http.ServeMux

	clients   map[string]*sseClient
	clientsMu sync.RWMutex

	activeConns int64
}

// NewHTTPServer creates a new HTTP/SSE MCP server.
func NewHTTPServer(
	name, version string,
	config HTTPServerConfig,
) *HTTPServer {
	s := &HTTPServer{
		info: protocol.ServerInfo{
			Name:    name,
			Version: version,
		},
		config:    config,
		tools:     make(map[string]*toolEntry),
		resources: make(map[string]*resourceEntry),
		prompts:   make(map[string]*promptEntry),
		clients:   make(map[string]*sseClient),
	}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/sse", s.handleSSE)
	s.mux.HandleFunc("/message", s.handleMessage)
	s.mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         config.Address,
		Handler:      s.mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return s
}

// RegisterTool registers a tool with a handler.
func (s *HTTPServer) RegisterTool(
	tool protocol.Tool,
	handler ToolHandler,
) {
	s.tools[tool.Name] = &toolEntry{tool: tool, handler: handler}
}

// RegisterResource registers a resource with a handler.
func (s *HTTPServer) RegisterResource(
	resource protocol.Resource,
	handler ResourceHandler,
) {
	s.resources[resource.URI] = &resourceEntry{
		resource: resource,
		handler:  handler,
	}
}

// RegisterPrompt registers a prompt with a handler.
func (s *HTTPServer) RegisterPrompt(
	prompt protocol.Prompt,
	handler PromptHandler,
) {
	s.prompts[prompt.Name] = &promptEntry{
		prompt:  prompt,
		handler: handler,
	}
}

// Serve starts the HTTP server and blocks until context is cancelled.
func (s *HTTPServer) Serve(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// Handler returns the HTTP handler for embedding in existing servers.
func (s *HTTPServer) Handler() http.Handler {
	return s.mux
}

// handleSSE handles SSE connection requests.
func (s *HTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(
			w, "Streaming not supported",
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	clientID := uuid.New().String()
	client := &sseClient{
		id:      clientID,
		writer:  w,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	s.clientsMu.Lock()
	s.clients[clientID] = client
	s.clientsMu.Unlock()
	atomic.AddInt64(&s.activeConns, 1)

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, clientID)
		s.clientsMu.Unlock()
		close(client.done)
		atomic.AddInt64(&s.activeConns, -1)
	}()

	_, _ = fmt.Fprintf(
		w, "event: connected\ndata: {\"clientId\":\"%s\"}\n\n", clientID,
	)

	messageEndpoint := fmt.Sprintf("http://%s/message", r.Host)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpoint)
	flusher.Flush()

	heartbeat := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, ":heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// handleMessage handles JSON-RPC message requests.
func (s *HTTPServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body := http.MaxBytesReader(w, r.Body, s.config.MaxRequestSize)
	data, err := io.ReadAll(body)
	if err != nil {
		s.writeError(
			w, nil, protocol.CodeParseError,
			"failed to read request body", nil,
		)
		return
	}

	var req protocol.Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.writeError(
			w, nil, protocol.CodeParseError, "invalid JSON", err.Error(),
		)
		return
	}

	if req.JSONRPC != protocol.JSONRPCVersion {
		s.writeError(
			w, req.ID, protocol.CodeInvalidRequest,
			"invalid JSON-RPC version", nil,
		)
		return
	}

	resp := handleRequest(
		r.Context(), &req, s.tools, s.resources, s.prompts, s.info,
	)

	if resp == nil {
		// Notification — no response needed
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	respData, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(respData)

	// Also broadcast to SSE clients
	s.broadcastToSSE(resp)
}

// handleHealth handles health check requests.
func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "healthy",
		"server":            s.info.Name,
		"version":           s.info.Version,
		"active_connections": atomic.LoadInt64(&s.activeConns),
		"tools":             len(s.tools),
		"resources":         len(s.resources),
		"prompts":           len(s.prompts),
	})
}

// broadcastToSSE sends a response to all connected SSE clients.
func (s *HTTPServer) broadcastToSSE(resp *protocol.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	s.clientsMu.RLock()
	clients := make([]*sseClient, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	s.clientsMu.RUnlock()

	for _, c := range clients {
		select {
		case <-c.done:
			continue
		default:
		}
		_, writeErr := fmt.Fprintf(
			c.writer, "event: message\ndata: %s\n\n", data,
		)
		if writeErr == nil {
			c.flusher.Flush()
		}
	}
}

// writeError writes a JSON-RPC error response.
func (s *HTTPServer) writeError(
	w http.ResponseWriter,
	id interface{},
	code int,
	message string,
	data interface{},
) {
	resp := protocol.NewErrorResponse(id, code, message, data)
	w.Header().Set("Content-Type", "application/json")
	respData, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(respData)
}

// ServerInfo returns the server identification.
func (s *HTTPServer) ServerInfo() protocol.ServerInfo {
	return s.info
}

// Capabilities returns the server capabilities.
func (s *HTTPServer) Capabilities() protocol.ServerCapabilities {
	caps := protocol.ServerCapabilities{}
	if len(s.tools) > 0 {
		caps.Tools = &protocol.ToolsCapability{}
	}
	if len(s.resources) > 0 {
		caps.Resources = &protocol.ResourcesCapability{}
	}
	if len(s.prompts) > 0 {
		caps.Prompts = &protocol.PromptsCapability{}
	}
	return caps
}
