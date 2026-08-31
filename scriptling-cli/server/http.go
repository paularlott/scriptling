package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/util"
)

// checkRouteConflicts registers every collected route into a throwaway mux
// so ServeMux's own conflict rules decide deterministically at startup: two
// wildcard-equivalent patterns (say /items/{name}/detail and
// /items/{slug}/detail) otherwise race map iteration order in buildMux, and
// whichever registers second was silently dropped.
func (s *Server) checkRouteConflicts() error {
	probe := http.NewServeMux()
	register := func(pattern string) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("route %q conflicts with another route: %v", pattern, r)
			}
		}()
		probe.Handle(pattern, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		return nil
	}

	// Match buildMux's protocol predicates and ordering so user routes cannot
	// silently lose to enabled built-in endpoints.
	if s.mcpEnabled() {
		if err := register("POST /mcp"); err != nil {
			return err
		}
		if err := register("GET /mcp"); err != nil {
			return err
		}
	}
	if s.config.JSONRPC {
		if err := register("POST /json-rpc"); err != nil {
			return err
		}
		if err := register("GET /json-rpc"); err != nil {
			return err
		}
	}

	for key := range s.handlers {
		pattern := key
		if strings.HasSuffix(key, " /") {
			pattern += "{$}"
		}
		if err := register(pattern); err != nil {
			return err
		}
	}
	for path := range s.wsHandlers {
		if err := register(path); err != nil {
			return err
		}
	}
	for path := range s.staticRoutes {
		if err := register(path); err != nil {
			return err
		}
	}
	return nil
}

// registerRoute adds one route pattern to the mux. Conflicts are caught up
// front by checkRouteConflicts; the recover stays as a last line of defense
// so a late registration cannot take the process down.
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
		mcp := s.scriptProtocolMiddleware(sseWriteDeadline(s.mcpHandler))
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
	// A configured static token always wraps the whole mux, middleware or
	// not: the script middleware guards the script-facing endpoints, but it
	// never runs for /health, static routes, the webroot fallback or custom
	// not-found handling, so dropping the token when middleware exists left
	// those unauthenticated. With both configured, the token applies first
	// and the middleware layers on top.
	if s.config.BearerToken != "" {
		handler = s.bearerTokenMiddleware(handler)
	}
	// Body caps apply outermost so no route, protocol endpoint or fallback
	// can buffer past the limit.
	handler = s.bodyLimitMiddleware(handler)
	return handler
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    s.config.Address,
		Handler: s.buildMux(),
		// Slowloris-style stalls and drip-fed bodies get cut off; the
		// read/write budgets are generous so legitimate large uploads and
		// slow handlers survive. Hijacked connections (WebSocket upgrades)
		// leave the net/http lifecycle, so these do not cut them.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
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

	// Bind synchronously so a bad address or a broken certificate fails
	// Start itself instead of surfacing asynchronously after a nil return.
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.config.Address, err)
	}
	useTLS := s.config.TLSGenerate || (s.config.TLSCert != "" && s.config.TLSKey != "")
	if useTLS {
		certFiles := []string{s.config.TLSCert, s.config.TLSKey}
		if s.config.TLSGenerate {
			certFiles = nil
		}
		cert, configErr := tlsConfigFor(s, certFiles)
		if configErr != nil {
			_ = listener.Close()
			return configErr
		}
		listener = tls.NewListener(listener, cert)
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			Log.Error("Server error", "error", err)
		}
	}()

	return nil
}

// tlsConfigFor builds the TLS configuration, loading the named certificate
// files when given (the self-signed path carries its certificate on the
// server config already).
func tlsConfigFor(s *Server, certFiles []string) (*tls.Config, error) {
	if len(certFiles) == 2 && certFiles[0] != "" && certFiles[1] != "" {
		cert, err := tls.LoadX509KeyPair(certFiles[0], certFiles[1])
		if err != nil {
			return nil, fmt.Errorf("load TLS keypair: %w", err)
		}
		cfg := tls.Config{MinVersion: tls.VersionTLS12}
		cfg.Certificates = []tls.Certificate{cert}
		return &cfg, nil
	}
	if s.httpServer.TLSConfig == nil {
		return nil, fmt.Errorf("no TLS configuration available")
	}
	return s.httpServer.TLSConfig.Clone(), nil
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

	body, ok := readBody(w, r)
	if !ok {
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	reqObj := s.createRequestObject(r, pathParams(r), body)
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
		reqObj := s.createRequestObject(r, nil, nil)
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
func (s *Server) createRequestObject(r *http.Request, pathParams map[string]string, body []byte) *object.Instance {

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

	return extlibs.CreateRequestInstance(r.Method, r.URL.Path, string(body), headers, query, pathParams, r.RemoteAddr)
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

	if err := p.ImportWithContext(ctx, libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return nil
	}

	result, err := p.CallFunctionWithContext(ctx, handlerRef, reqObj)
	if err != nil {
		Log.Error("Handler error", "error", err)
		return object.NewStringDict(map[string]object.Object{
			"status":  object.NewInteger(500),
			"headers": &object.Dict{Pairs: map[string]object.DictPair{}},
			"body":    object.NewString(extlibs.ErrorJSONBody(err.Error())),
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
			var ok bool
			if bodyBytes, ok = readBody(w, r); !ok {
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		reqObj := s.createRequestObject(r, nil, bodyBytes)
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
					"body":    object.NewString(extlibs.ErrorJSONBody(err.Error())),
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

// DefaultMaxRequestBody is the per-request body cap applied when
// ServerConfig.MaxRequestBodyBytes is unset. Generous enough for large
// JSON-RPC batches and uploads, small enough that one hostile request cannot
// buffer unbounded memory.
const DefaultMaxRequestBody int64 = 32 << 20 // 32 MiB

// maxRequestBody resolves the effective per-request body limit; negative
// disables the cap for embedders that stream arbitrarily large uploads.
func (s *Server) maxRequestBody() int64 {
	if s.config.MaxRequestBodyBytes != 0 {
		return s.config.MaxRequestBodyBytes
	}
	return DefaultMaxRequestBody
}

// sseWriteDeadline clears the server's write deadline for GET requests on the
// MCP endpoint: the GET is an SSE stream, a response with no end, and the
// server-wide WriteTimeout would cut subscribers off mid-stream.
func sseWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		}
		next.ServeHTTP(w, r)
	})
}

// readBody reads the request body under the middleware's cap. A body past
// the cap answers 413 and reports false: handlers must never see a truncated
// payload as if it were the whole request.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return nil, false
		}
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

// bodyLimitMiddleware bounds request bodies. Handlers that read past the cap
// see MaxBytesReader's error and answer 4xx instead of buffering whatever a
// client cares to send.
func (s *Server) bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limit := s.maxRequestBody(); limit > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}
