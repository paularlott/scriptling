package server

import (
	"net/http"
	"strings"
)

// handlerForRequest resolves the handler reference for a request the mux
// routed to handleScriptRequest. ServeMux reports the matched pattern via
// r.Pattern (Go 1.23+) in the same "METHOD path" form used as the handler
// map key, so the mux stays the single source of matching truth — wildcard
// precedence, method dispatch (including HEAD→GET), subtree patterns, and
// escaping are all handled there. The one wrinkle is bare "/" routes:
// buildMux registers them as "/{$}" while the map key keeps "/", so both
// spellings are tried.
func (s *Server) handlerForRequest(r *http.Request) (string, bool) {
	pattern := r.Pattern
	if pattern == "" {
		// handleScriptRequest mounted outside a ServeMux: exact literal key only.
		pattern = r.Method + " " + r.URL.EscapedPath()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ref, ok := s.handlers[pattern]; ok {
		return ref, true
	}
	ref, ok := s.handlers[strings.TrimSuffix(pattern, "{$}")]
	return ref, ok
}

// pathParams extracts the wildcard values captured by the matched pattern
// (e.g. "/api/users/{id}", "/files/{path...}") using the mux's own
// percent-decoding, mirroring r.PathValue.
func pathParams(r *http.Request) map[string]string {
	if !strings.Contains(r.Pattern, "{") {
		return nil
	}
	_, patternPath, _ := strings.Cut(r.Pattern, " ")
	var params map[string]string
	for _, seg := range strings.Split(patternPath, "/") {
		if seg == "{$}" || !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
			continue
		}
		name := strings.TrimSuffix(seg[1:len(seg)-1], "...")
		if params == nil {
			params = make(map[string]string, 1)
		}
		params[name] = r.PathValue(name)
	}
	return params
}
