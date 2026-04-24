package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
	"github.com/paularlott/scriptling/util"
)

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()

	if s.mcpHandler != nil {
		mux.Handle("/mcp", s.mcpHandler)
	}

	mux.HandleFunc("GET /health", s.handleHealth)

	for key := range s.handlers {
		mux.HandleFunc(key, s.handleScriptRequest)
	}

	for path := range s.wsHandlers {
		mux.HandleFunc(path, s.handleScriptRequest)
	}

	for path, dir := range s.staticRoutes {
		fs := http.FileServer(http.Dir(dir))
		mux.Handle(path, http.StripPrefix(path, fs))
	}

	// Web root: serve files not matched by any route, fall through to 404 handler
	if s.config.WebRoot != "" || s.notFoundHandler != "" {
		mux.HandleFunc("/", s.handleFallback)
	}

	var handler http.Handler = mux
	if s.config.BearerToken != "" {
		if s.middleware == "" {
			handler = s.bearerTokenMiddleware(mux)
		} else {
			handler = s.bearerTokenMCPOnlyMiddleware(mux)
		}
	}

	s.httpServer = &http.Server{
		Addr:    s.config.Address,
		Handler: handler,
	}

	if s.config.TLSGenerate || (s.config.TLSCert != "" && s.config.TLSKey != "") {
		if s.config.TLSGenerate {
			cert, err := s.generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("failed to generate certificate: %w", err)
			}
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			Log.Info("Using self-signed certificate")
		} else {
			s.httpServer.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
	}

	go func() {
		var err error

		if s.config.TLSGenerate || (s.config.TLSCert != "" && s.config.TLSKey != "") {
			if s.config.TLSCert != "" && s.config.TLSKey != "" {
				err = s.httpServer.ListenAndServeTLS(s.config.TLSCert, s.config.TLSKey)
			} else {
				err = s.httpServer.ListenAndServeTLS("", "")
			}
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			Log.Error("Server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	extlibs.RuntimeState.Lock()
	conns := make([]*extlibs.WebSocketServerConn, 0, len(extlibs.RuntimeState.WebSocketConnections))
	for _, conn := range extlibs.RuntimeState.WebSocketConnections {
		conns = append(conns, conn)
	}
	extlibs.RuntimeState.WebSocketConnections = make(map[string]*extlibs.WebSocketServerConn)
	extlibs.RuntimeState.Unlock()

	for _, conn := range conns {
		conn.Close()
	}

	return s.httpServer.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "OK")
}

// handleScriptRequest handles requests to script handlers
func (s *Server) handleScriptRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if isWebSocketRequest(r) {
		s.mu.RLock()
		_, isWS := s.wsHandlers[path]
		s.mu.RUnlock()

		if isWS {
			s.handleWebSocketUpgrade(w, r, path)
			return
		}
	}

	// Look up handler by "METHOD path"
	key := r.Method + " " + path
	s.mu.RLock()
	handlerRef, ok := s.handlers[key]
	if !ok && !strings.HasSuffix(path, "/") {
		handlerRef, ok = s.handlers[key+"/"]
	}
	s.mu.RUnlock()

	if !ok {
		s.serveNotFound(w, r)
		return
	}

	reqObj := s.createRequestObject(r)

	if s.middleware != "" {
		if resp := s.runHandler(s.middleware, reqObj); resp != nil {
			s.writeResponse(w, resp)
			return
		}
	}

	if resp := s.runHandler(handlerRef, reqObj); resp != nil {
		s.writeResponse(w, resp)
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleFallback serves files from WebRoot or calls the not_found handler
func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	if s.config.WebRoot != "" {
		// Resolve the web root to an absolute path once
		webRoot, err := filepath.Abs(s.config.WebRoot)
		if err != nil {
			s.serveNotFound(w, r)
			return
		}

		// Build candidate path and resolve to absolute to prevent traversal
		urlPath := filepath.FromSlash(r.URL.Path)
		candidate, err := filepath.Abs(filepath.Join(webRoot, urlPath))
		if err != nil || !strings.HasPrefix(candidate, webRoot+string(filepath.Separator)) && candidate != webRoot {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Try the path directly, then index.html for directory-like requests
		for _, p := range []string{candidate, filepath.Join(candidate, "index.html")} {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				http.ServeFile(w, r, p)
				return
			}
		}
	}
	s.serveNotFound(w, r)
}

// serveNotFound calls the not_found handler or returns a plain 404
func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	if s.notFoundHandler != "" {
		reqObj := s.createRequestObject(r)
		if resp := s.runHandler(s.notFoundHandler, reqObj); resp != nil {
			s.writeResponse(w, resp)
			return
		}
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

// createRequestObject creates a Request instance from an HTTP request
func (s *Server) createRequestObject(r *http.Request) *object.Instance {
	var body string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		body = string(bodyBytes)
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	query := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	return extlibs.CreateRequestInstance(r.Method, r.URL.Path, body, headers, query)
}

// runHandler runs a handler function and returns the response
func (s *Server) runHandler(handlerRef string, reqObj *object.Instance) *object.Dict {
	libName, _, ok := strings.Cut(handlerRef, ".")
	if !ok {
		Log.Error("Invalid handler reference", "handler", handlerRef)
		return nil
	}

	p := scriptling.New()
	setup.Scriptling(p, s.config.LibDirs, false, s.config.AllowedPaths, s.config.DisabledLibs, s.config.SecretRegistry, Log, s.config.DockerSock, s.config.PodmanSock)
	s.applyPackLoader(p)

	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return nil
	}

	result, err := p.CallFunction(handlerRef, reqObj)
	if err != nil {
		Log.Error("Handler error", "error", err)
		return object.NewStringDict(map[string]object.Object{
			"status":  object.NewInteger(500),
			"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
			"body":    &object.String{Value: fmt.Sprintf(`{"error": "%s"}`, err.Error())},
		})
	}

	if dict, ok := result.(*object.Dict); ok {
		return dict
	}

	// Null means "no response" (e.g. middleware returning None to continue)
	if _, ok := result.(*object.Null); ok {
		return nil
	}

	return object.NewStringDict(map[string]object.Object{
		"status":  object.NewInteger(200),
		"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
		"body":    result,
	})
}

// writeResponse writes a response dict to the HTTP response writer
func (s *Server) writeResponse(w http.ResponseWriter, resp *object.Dict) {
	status := int64(200)
	if statusObj, ok := resp.GetByString("status"); ok {
		if statusInt, err := statusObj.Value.AsInt(); err == nil {
			status = statusInt
		}
	}

	if headersObj, ok := resp.GetByString("headers"); ok {
		if headersDict, err := headersObj.Value.AsDict(); err == nil {
			for k, v := range headersDict {
				if strVal, err := v.AsString(); err == nil {
					w.Header().Set(k, strVal)
				}
			}
		}
	}

	var bodyBytes []byte
	if bodyObj, ok := resp.GetByString("body"); ok {
		if strVal, err := bodyObj.Value.AsString(); err == nil {
			bodyBytes = []byte(strVal)
		} else {
			jsonBytes, err := json.Marshal(objectToInterface(bodyObj.Value))
			if err != nil {
				Log.Error("Failed to encode JSON response", "error", err)
				bodyBytes = []byte(`{"error": "JSON encoding failed"}`)
			} else {
				bodyBytes = jsonBytes
				if w.Header().Get("Content-Type") == "" {
					w.Header().Set("Content-Type", "application/json")
				}
			}
		}
	}

	w.WriteHeader(int(status))
	w.Write(bodyBytes)
}

// objectToInterface converts a scriptling Object to a Go interface{}
func objectToInterface(obj object.Object) interface{} {
	switch v := obj.(type) {
	case *object.String:
		return v.Value
	case *object.Integer:
		return v.Value
	case *object.Float:
		return v.Value
	case *object.Boolean:
		return v.Value
	case *object.Null:
		return nil
	case *object.List:
		result := make([]interface{}, len(v.Elements))
		for i, elem := range v.Elements {
			result[i] = objectToInterface(elem)
		}
		return result
	case *object.Dict:
		result := make(map[string]interface{})
		for _, pair := range v.Pairs {
			result[pair.StringKey()] = objectToInterface(pair.Value)
		}
		return result
	default:
		return nil
	}
}

// bearerTokenMiddleware creates authentication middleware for all endpoints
func (s *Server) bearerTokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != s.bearerExpected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerTokenMCPOnlyMiddleware creates authentication middleware for MCP only
func (s *Server) bearerTokenMCPOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mcp" && r.Header.Get("Authorization") != s.bearerExpected {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// generateSelfSignedCert generates a self-signed certificate
func (s *Server) generateSelfSignedCert() (tls.Certificate, error) {
	hosts := util.GetCertificateHosts(s.config.Address)
	return util.GenerateSelfSignedCertificate(util.CertificateConfig{
		Hosts: hosts,
	})
}
