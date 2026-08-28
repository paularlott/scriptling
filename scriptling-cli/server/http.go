package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/util"
)

// registerRoute adds one route pattern to the mux. ServeMux panics on
// patterns it considers conflicting (two wildcards with the same shape, such
// as /items/{name}/detail and /items/{slug}/detail, or the same pattern
// registered twice); rather than crashing the server at startup, skip the
// route with an error log so the rest of the app still serves.
func registerRoute(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	defer func() {
		if r := recover(); r != nil {
			Log.Error("Skipping conflicting route pattern", "pattern", pattern, "reason", fmt.Sprintf("%v", r))
		}
	}()
	mux.HandleFunc(pattern, handler)
}

// buildMux assembles the full HTTP handler stack: protocol endpoints, script
// routes, static routes, web root fallback, and auth middleware.
func (s *Server) buildMux() http.Handler {
	mux := http.NewServeMux()

	if s.mcpHandler != nil {
		mcp := s.scriptProtocolMiddleware(s.mcpHandler)
		mux.Handle("POST /mcp", mcp)
		mux.Handle("GET /mcp", mcp)
	}
	if s.config.JSONRPC {
		if s.pluginServer != nil {
			jr := s.scriptProtocolMiddleware(s.pluginServer)
			mux.Handle("POST /json-rpc", jr)
			mux.Handle("GET /json-rpc", jr)
		} else {
			jr := s.scriptProtocolMiddleware(http.HandlerFunc(s.handleJSONRPCHTTP))
			mux.Handle("POST /json-rpc", jr)
			mux.Handle("GET /json-rpc", jr)
		}
	}

	// Built-in health check — skip if a user route already claims it.
	if _, ok := s.handlers["GET /health"]; !ok {
		mux.HandleFunc("GET /health", s.handleHealth)
	}

	for key := range s.handlers {
		// "GET /" creates a subtree pattern in Go 1.22's mux that would swallow
		// all GET requests. Append {$} so it matches exactly "/" and lets other
		// paths fall through to the webroot fallback.
		if strings.HasSuffix(key, " /") {
			registerRoute(mux, key+"{$}", s.handleScriptRequest)
		} else {
			registerRoute(mux, key, s.handleScriptRequest)
		}
	}

	for path := range s.wsHandlers {
		mux.HandleFunc(path, s.handleScriptRequest)
	}

	for path, dir := range s.staticRoutes {
		fs := http.FileServer(http.Dir(dir))
		mux.Handle(path, http.StripPrefix(path, fs))
	}

	// Web root: serve files not matched by any route, fall through to 404 handler
	if s.config.WebRoot != "" || s.notFoundHandler != "" || s.webRootFS != nil {
		mux.HandleFunc("/", s.handleFallback)
	}

	var handler http.Handler = mux
	// With a script middleware registered, it guards every endpoint — routes
	// and WebSocket upgrades via handleScriptRequest and the protocol
	// endpoints via scriptProtocolMiddleware — so the static bearer token is
	// not applied. Without one, a configured static token guards everything.
	if s.config.BearerToken != "" && s.middleware == "" {
		handler = s.bearerTokenMiddleware(mux)
	}
	return handler
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.config.Address,
		Handler: s.buildMux(),
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

	if s.webRootZip != nil {
		s.webRootZip.Close()
	}

	return s.httpServer.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	Log.Trace("HTTP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	io.WriteString(w, "OK")
}

// handleScriptRequest handles requests to script handlers
func (s *Server) handleScriptRequest(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	Log.Trace("HTTP request", "method", r.Method, "path", path, "remote", r.RemoteAddr)

	if isWebSocketRequest(r) {
		s.mu.RLock()
		_, isWS := s.wsHandlers[path]
		s.mu.RUnlock()

		if isWS {
			s.handleWebSocketUpgrade(w, r, path)
			return
		}
	}

	// The mux already matched this request to a registered pattern; that
	// pattern is the handler map key.
	handlerRef, ok := s.handlerForRequest(r)
	if !ok {
		Log.Trace("No matching route", "method", r.Method, "path", path)
		s.serveNotFound(w, r)
		return
	}

	reqObj := s.createRequestObject(r, pathParams(r))
	ctx := extlibs.WithRequestContext(r.Context(), reqObj)

	if s.middleware != "" {
		Log.Trace("Running middleware", "handler", s.middleware)
		if resp := s.runHandler(ctx, s.middleware, reqObj); resp != nil {
			s.writeResponse(w, resp)
			return
		}
	}

	Log.Trace("Dispatching to handler", "handler", handlerRef)
	if resp := s.runHandler(ctx, handlerRef, reqObj); resp != nil {
		s.writeResponse(w, resp)
	} else {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleFallback serves files from WebRoot (directory or zip), an app bundle's
// webroot/, or calls the not_found handler
func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	Log.Trace("HTTP fallback request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	if s.webRootFS != nil {
		s.serveFromBundle(w, r)
		return
	}
	if s.config.WebRoot != "" {
		if s.webRootZip != nil {
			s.serveFromZip(w, r)
			return
		}
		s.serveFromDir(w, r)
		return
	}
	s.serveNotFound(w, r)
}

// serveFromBundle serves static assets from the app bundle's cached webroot/ FS.
func (s *Server) serveFromBundle(w http.ResponseWriter, r *http.Request) {
	if s.webRootFS == nil {
		s.serveNotFound(w, r)
		return
	}

	// Normalise the URL path: strip leading slash, never allow traversal.
	urlPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if urlPath == "." {
		urlPath = ""
	}
	candidates := []string{urlPath, urlPath + "/index.html"}
	if urlPath == "" {
		candidates = []string{"index.html"}
	}

	for _, candidate := range candidates {
		if !fs.ValidPath(candidate) {
			continue
		}
		info, err := fs.Stat(s.webRootFS, candidate)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := fs.ReadFile(s.webRootFS, candidate)
		if err != nil {
			continue
		}
		Log.Trace("Serving file from bundle webroot", "file", candidate)
		if ct := mime.TypeByExtension(path.Ext(candidate)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Write(data)
		return
	}
	Log.Trace("Bundle webroot entry not found", "path", urlPath)
	s.serveNotFound(w, r)
}

// serveFromDir serves a file from the web root directory. Containment is
// provided by os.DirFS: candidate paths are validated with fs.ValidPath and
// resolved relative to webRoot only, so ".." and absolute paths can never
// escape it. Mirrors serveFromBundle.
func (s *Server) serveFromDir(w http.ResponseWriter, r *http.Request) {
	webRoot, err := filepath.Abs(s.config.WebRoot)
	if err != nil {
		Log.Debug("Web root resolve failed", "web_root", s.config.WebRoot, "error", err)
		s.serveNotFound(w, r)
		return
	}
	rootFS := os.DirFS(webRoot)

	// Normalise the URL path: strip leading slash, never allow traversal.
	urlPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if urlPath == "." {
		urlPath = ""
	}
	candidates := []string{urlPath, urlPath + "/index.html"}
	if urlPath == "" {
		candidates = []string{"index.html"}
	}

	for _, candidate := range candidates {
		if !fs.ValidPath(candidate) {
			continue
		}
		info, err := fs.Stat(rootFS, candidate)
		if err != nil || info.IsDir() {
			continue
		}
		data, err := fs.ReadFile(rootFS, candidate)
		if err != nil {
			continue
		}
		Log.Trace("Serving file from web root", "file", candidate)
		if ct := mime.TypeByExtension(path.Ext(candidate)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		w.Write(data)
		return
	}
	Log.Trace("Web root file not found", "web_root", webRoot, "path", urlPath)
	s.serveNotFound(w, r)
}

// serveFromZip serves a file from the web root zip archive
func (s *Server) serveFromZip(w http.ResponseWriter, r *http.Request) {
	// Normalise the URL path: strip leading slash, never allow traversal
	urlPath := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if urlPath == "." {
		urlPath = ""
	}

	candidates := []string{urlPath, urlPath + "/index.html"}
	if urlPath == "" {
		candidates = []string{"index.html"}
	}

	for _, candidate := range candidates {
		for _, f := range s.webRootZip.File {
			if f.Name == candidate && !f.FileInfo().IsDir() {
				Log.Trace("Serving file from web root zip", "file", f.Name)
				rc, err := f.Open()
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(f.Name)))
				io.Copy(w, rc)
				rc.Close()
				return
			}
		}
	}
	Log.Trace("Web root zip entry not found", "path", urlPath)
	s.serveNotFound(w, r)
}

// serveNotFound calls the not_found handler or returns a plain 404
func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	if s.notFoundHandler != "" {
		Log.Trace("Handling 404 via not_found handler", "handler", s.notFoundHandler, "path", r.URL.Path)
		reqObj := s.createRequestObject(r, nil)
		ctx := extlibs.WithRequestContext(r.Context(), reqObj)
		if resp := s.runHandler(ctx, s.notFoundHandler, reqObj); resp != nil {
			s.writeResponse(w, resp)
			return
		}
	}
	Log.Trace("Returning 404", "method", r.Method, "path", r.URL.Path)
	http.Error(w, "Not Found", http.StatusNotFound)
}

// createRequestObject creates a Request instance from an HTTP request.
// pathParams holds values captured from route wildcards, already unescaped.
func (s *Server) createRequestObject(r *http.Request, pathParams map[string]string) *object.Instance {
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

	return extlibs.CreateRequestInstance(r.Method, r.URL.Path, body, headers, query, pathParams, r.RemoteAddr)
}

// splitHandlerRef splits a "module.function" handler reference at the last
// dot: module names may themselves be dotted (a module in a subdirectory,
// such as "routes.me"), while the function or class name is always a single
// identifier. Cutting at the first dot instead would try to import "routes"
// for a "routes.me.me" reference, which is not a module.
func splitHandlerRef(ref string) (module, fn string, ok bool) {
	idx := strings.LastIndex(ref, ".")
	if idx <= 0 || idx == len(ref)-1 {
		return "", "", false
	}
	return ref[:idx], ref[idx+1:], true
}

// runHandler runs a handler function and returns the response
func (s *Server) runHandler(ctx context.Context, handlerRef string, reqObj *object.Instance) *object.Dict {
	libName, _, ok := splitHandlerRef(handlerRef)
	if !ok {
		Log.Error("Invalid handler reference", "handler", handlerRef)
		return nil
	}

	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return nil
	}

	result, err := p.CallFunctionWithContext(ctx, handlerRef, reqObj)
	if err != nil {
		Log.Error("Handler error", "error", err)
		return object.NewStringDict(map[string]object.Object{
			"status":  object.NewInteger(500),
			"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
			"body":    object.NewString(fmt.Sprintf(`{"error": "%s"}`, err.Error())),
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
			jsonBytes, err := json.Marshal(conversion.ToGo(bodyObj.Value))
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

	Log.Trace("HTTP response", "status", status, "bytes", len(bodyBytes))
	w.WriteHeader(int(status))
	w.Write(bodyBytes)
}

// scriptProtocolMiddleware wraps the protocol endpoints (/mcp, /json-rpc).
// The originating HTTP request is always stashed on the request context so
// tool and method handlers can query it (scriptling.mcp.tool.get_request(),
// runtime.jsonrpc.get_request()). When a script middleware is registered it
// then runs with the request object: a returned response dict blocks the
// request (e.g. a 401), None lets it through to the protocol handler. The
// MCP entries the middleware registered for this request (register_request_
// tool / _resource / _prompt) become per-request providers, so tools/list and
// tools/call see exactly the entries that request's middleware exposed —
// per-user tool sets with authorization re-evaluated on every message. The
// request body is buffered and restored, so building the middleware's request
// object does not consume it for the protocol handler that runs next.
func (s *Server) scriptProtocolMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		reqObj := s.createRequestObject(r, nil)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		r = r.WithContext(extlibs.WithRequestContext(r.Context(), reqObj))

		if s.middleware != "" {
			Log.Trace("Running middleware", "handler", s.middleware, "path", r.URL.Path)
			if resp := s.runHandler(r.Context(), s.middleware, reqObj); resp != nil {
				s.writeResponse(w, resp)
				return
			}
		}

		// Build the per-request MCP providers from what the middleware
		// registered. A malformed registration is a build error: fail the
		// request rather than serving a different tool set than intended.
		if regs := extlibs.RegistrationsFrom(r.Context()); regs != nil && !regs.Empty() {
			providers, err := s.buildRequestProviders(regs)
			if err != nil {
				Log.Error("Request MCP registration failed", "error", err)
				s.writeResponse(w, object.NewStringDict(map[string]object.Object{
					"status":  object.NewInteger(500),
					"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
					"body":    object.NewString(fmt.Sprintf(`{"error": "%s"}`, err.Error())),
				}))
				return
			}
			if providers.tools != nil {
				r = r.WithContext(mcplib.WithToolProviders(r.Context(), providers.tools))
			}
			if providers.resources != nil {
				r = r.WithContext(mcplib.WithResourceProviders(r.Context(), providers.resources))
			}
			if providers.prompts != nil {
				r = r.WithContext(mcplib.WithPromptProviders(r.Context(), providers.prompts))
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requestProviders holds the per-request MCP providers built from middleware
// registrations; nil members were not registered for this request.
type requestProviders struct {
	tools     mcplib.ToolProvider
	resources mcplib.ResourceProvider
	prompts   mcplib.PromptProvider
}

func (s *Server) buildRequestProviders(regs *extlibs.RequestRegistrations) (requestProviders, error) {
	var out requestProviders
	cfg := s.handlerConfig()

	if len(regs.Tools) > 0 {
		p, err := mcpcli.BuildRequestToolProvider(regs.Tools, cfg)
		if err != nil {
			return out, err
		}
		out.tools = p
	}
	if len(regs.Resources) > 0 {
		p, err := mcpcli.BuildRequestResourceProvider(regs.Resources, cfg)
		if err != nil {
			return out, err
		}
		out.resources = p
	}
	if len(regs.Prompts) > 0 {
		p, err := mcpcli.BuildRequestPromptProvider(regs.Prompts, cfg)
		if err != nil {
			return out, err
		}
		out.prompts = p
	}
	return out, nil
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

// generateSelfSignedCert generates a self-signed certificate
func (s *Server) generateSelfSignedCert() (tls.Certificate, error) {
	hosts := util.GetCertificateHosts(s.config.Address)
	return util.GenerateSelfSignedCertificate(util.CertificateConfig{
		Hosts: hosts,
	})
}
