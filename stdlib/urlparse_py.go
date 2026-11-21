package stdlib

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// urllib.parse's urlparse, urlsplit, urlunparse, urlunsplit, urljoin and
// urldefrag, following CPython's algorithms (not Go's net/url, which
// normalizes and rejects what Python keeps): results are named tuples with
// hostname, port, username and password properties, geturl() and
// _replace().

var (
	usesNetloc   = []string{"", "ftp", "http", "gopher", "nntp", "telnet", "imap", "wais", "file", "mms", "https", "shttp", "snews", "prospero", "rtsp", "rtsps", "rtspu", "rsync", "svn", "svn+ssh", "sftp", "nfs", "git", "git+ssh", "ws", "wss", "itms-services"}
	usesParams   = []string{"", "ftp", "hdl", "prospero", "http", "imap", "https", "shttp", "rtsp", "rtsps", "rtspu", "sip", "sips", "mms", "sftp", "tel"}
	usesRelative = []string{"", "ftp", "http", "gopher", "nntp", "imap", "wais", "file", "https", "shttp", "mms", "prospero", "rtsp", "rtsps", "rtspu", "sftp", "svn", "svn+ssh", "ws", "wss"}
)

var (
	parseResultFields  = []string{"scheme", "netloc", "path", "params", "query", "fragment"}
	splitResultFields  = []string{"scheme", "netloc", "path", "query", "fragment"}
	defragResultFields = []string{"url", "fragment"}
)

// SplitResultClass and DefragResultClass are the classes of urlsplit() and
// urldefrag() results (ParseResultClass, in urllib.go, is urlparse()'s).
var (
	SplitResultClass  = &object.Class{Name: "SplitResult", Methods: map[string]object.Object{}}
	DefragResultClass = &object.Class{Name: "DefragResult", Methods: map[string]object.Object{}}
)

func urlResultFields(c *object.Class) []string {
	switch c {
	case SplitResultClass:
		return splitResultFields
	case DefragResultClass:
		return defragResultFields
	}
	return parseResultFields
}

func newURLResult(c *object.Class, values ...string) *object.Instance {
	fields := map[string]object.Object{}
	for i, name := range urlResultFields(c) {
		fields[name] = object.NewString(values[i])
	}
	return object.NewInstanceWithFields(c, fields)
}

func urlResultValues(inst *object.Instance) []string {
	names := urlResultFields(inst.Class)
	out := make([]string, len(names))
	for i, n := range names {
		if s, ok := inst.Field(n).(*object.String); ok {
			out[i] = s.StringValue()
		}
	}
	return out
}

// pyURLSplit is urllib.parse.urlsplit.
func pyURLSplit(url, scheme string, allowFragments bool) ([5]string, object.Object) {
	url = strings.TrimLeft(url, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	url = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(url)
	if i := strings.IndexByte(url, ':'); i > 0 && isASCIILetter(url[0]) {
		ok := true
		for j := 0; j < i; j++ {
			c := url[j]
			if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				ok = false
				break
			}
		}
		if ok {
			scheme, url = strings.ToLower(url[:i]), url[i+1:]
		}
	}
	netloc, query, fragment := "", "", ""
	if strings.HasPrefix(url, "//") {
		delim := len(url)
		for _, c := range "/?#" {
			if w := strings.IndexRune(url[2:], c); w >= 0 && w+2 < delim {
				delim = w + 2
			}
		}
		netloc, url = url[2:delim], url[delim:]
		if strings.Contains(netloc, "[") != strings.Contains(netloc, "]") {
			return [5]string{}, errors.NewValueError("Invalid IPv6 URL")
		}
	}
	if allowFragments {
		if i := strings.IndexByte(url, '#'); i >= 0 {
			url, fragment = url[:i], url[i+1:]
		}
	}
	if i := strings.IndexByte(url, '?'); i >= 0 {
		url, query = url[:i], url[i+1:]
	}
	return [5]string{scheme, netloc, url, query, fragment}, nil
}

// pyURLParse is urllib.parse.urlparse: urlsplit plus ;params.
func pyURLParse(url, scheme string, allowFragments bool) ([6]string, object.Object) {
	s, errObj := pyURLSplit(url, scheme, allowFragments)
	if errObj != nil {
		return [6]string{}, errObj
	}
	path, params := s[2], ""
	if slices.Contains(usesParams, s[0]) && strings.Contains(path, ";") {
		i := -1
		if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
			if j := strings.IndexByte(path[slash:], ';'); j >= 0 {
				i = slash + j
			}
		} else {
			i = strings.IndexByte(path, ';')
		}
		if i >= 0 {
			path, params = path[:i], path[i+1:]
		}
	}
	return [6]string{s[0], s[1], path, params, s[3], s[4]}, nil
}

// pyURLUnsplit is urllib.parse.urlunsplit.
func pyURLUnsplit(scheme, netloc, url, query, fragment string) string {
	switch {
	case netloc != "":
		if url != "" && url[0] != '/' {
			url = "/" + url
		}
		url = "//" + netloc + url
	case strings.HasPrefix(url, "//"):
		url = "//" + url
	case scheme != "" && slices.Contains(usesNetloc, scheme) && (url == "" || url[0] == '/'):
		url = "//" + url
	}
	if scheme != "" {
		url = scheme + ":" + url
	}
	if query != "" {
		url += "?" + query
	}
	if fragment != "" {
		url += "#" + fragment
	}
	return url
}

func pyURLUnparse(p [6]string) string {
	path := p[2]
	if p[3] != "" {
		path += ";" + p[3]
	}
	return pyURLUnsplit(p[0], p[1], path, p[4], p[5])
}

// pyURLJoin is urllib.parse.urljoin.
func pyURLJoin(base, url string, allowFragments bool) (string, object.Object) {
	if base == "" {
		return url, nil
	}
	if url == "" {
		return base, nil
	}
	b, errObj := pyURLParse(base, "", allowFragments)
	if errObj != nil {
		return "", errObj
	}
	u, errObj := pyURLParse(url, b[0], allowFragments)
	if errObj != nil {
		return "", errObj
	}
	scheme, netloc, path, params, query, fragment := u[0], u[1], u[2], u[3], u[4], u[5]
	if scheme != b[0] || !slices.Contains(usesRelative, scheme) {
		return url, nil
	}
	if slices.Contains(usesNetloc, scheme) {
		if netloc != "" {
			return pyURLUnparse(u), nil
		}
		netloc = b[1]
	}
	if path == "" && params == "" {
		path, params = b[2], b[3]
		if query == "" {
			query = b[4]
		}
		return pyURLUnparse([6]string{scheme, netloc, path, params, query, fragment}), nil
	}
	baseParts := strings.Split(b[2], "/")
	if baseParts[len(baseParts)-1] != "" {
		baseParts = baseParts[:len(baseParts)-1]
	}
	var segments []string
	if strings.HasPrefix(path, "/") {
		segments = strings.Split(path, "/")
	} else {
		segments = append(baseParts, strings.Split(path, "/")...)
		if len(segments) > 2 {
			kept := []string{segments[0]}
			for _, s := range segments[1 : len(segments)-1] {
				if s != "" {
					kept = append(kept, s)
				}
			}
			segments = append(kept, segments[len(segments)-1])
		}
	}
	var resolved []string
	for _, seg := range segments {
		switch seg {
		case "..":
			if len(resolved) > 0 {
				resolved = resolved[:len(resolved)-1]
			}
		case ".":
		default:
			resolved = append(resolved, seg)
		}
	}
	if last := segments[len(segments)-1]; last == "." || last == ".." {
		resolved = append(resolved, "")
	}
	joined := strings.Join(resolved, "/")
	if joined == "" {
		joined = "/"
	}
	return pyURLUnparse([6]string{scheme, netloc, joined, params, query, fragment}), nil
}

// urlArgs reads (url, scheme=”, allow_fragments=True).
func urlArgs(name string, args []object.Object, kwargs object.Kwargs) (string, string, bool, object.Object) {
	if kwargs.Len() > 0 {
		if err := errors.CheckKwargs(name, kwargs, "url", "urlstring", "scheme", "allow_fragments"); err != nil {
			return "", "", false, err
		}
	}
	vals := map[string]object.Object{}
	keys := []string{"url", "scheme", "allow_fragments"}
	if len(args) > len(keys) {
		return "", "", false, errors.NewTypeErrorTagged("%s() takes at most 3 arguments (%d given)", name, len(args))
	}
	for i, a := range args {
		vals[keys[i]] = a
	}
	for k, v := range kwargs.All() {
		if k == "urlstring" {
			k = "url"
		}
		vals[k] = v
	}
	urlObj, ok := vals["url"]
	if !ok {
		return "", "", false, errors.NewTypeErrorTagged("%s() missing required argument: 'url'", name)
	}
	var url string
	switch u := urlObj.(type) {
	case *object.String:
		url = u.StringValue()
	case *object.Bytes:
		url = string(u.BytesValue())
	default:
		return "", "", false, errors.NewTypeErrorTagged("%s() argument must be str, not %s", name, object.PythonTypeName(urlObj))
	}
	scheme := ""
	if s, ok := vals["scheme"]; ok {
		str, isStr := s.(*object.String)
		if !isStr {
			return "", "", false, errors.NewTypeErrorTagged("scheme must be str")
		}
		scheme = str.StringValue()
	}
	allow := true
	if a, ok := vals["allow_fragments"]; ok {
		allow = isTruthy(a)
	}
	return url, scheme, allow, nil
}

// urlComponents reads a sequence (or result) of n strings.
func urlComponents(ctx context.Context, o object.Object, n int) ([]string, object.Object) {
	if inst, ok := o.(*object.Instance); ok && (inst.Class == ParseResultClass || inst.Class == SplitResultClass) {
		vals := urlResultValues(inst)
		if len(vals) == n {
			return vals, nil
		}
	}
	elems, errObj := iterableElements(ctx, o)
	if errObj != nil {
		return nil, errObj
	}
	if len(elems) != n {
		return nil, errors.NewValueError("not enough values to unpack (expected %d, got %d)", n, len(elems))
	}
	out := make([]string, n)
	for i, e := range elems {
		switch v := e.(type) {
		case *object.String:
			out[i] = v.StringValue()
		case *object.Null:
		default:
			return nil, errors.NewTypeErrorTagged("Cannot mix str and non-str arguments")
		}
	}
	return out, nil
}

// netlocParts splits a netloc into userinfo and host:port, as the
// properties of a result read them.
func netlocHostinfo(netloc string) (userinfo string, hasUser bool, hostinfo string) {
	if i := strings.LastIndexByte(netloc, '@'); i >= 0 {
		return netloc[:i], true, netloc[i+1:]
	}
	return "", false, netloc
}

func urlResultProperty(name string) *object.Property {
	return &object.Property{Getter: &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			inst, ok := args[0].(*object.Instance)
			if !ok {
				return errors.NewTypeErrorTagged("%s needs a URL result", name)
			}
			netloc, _ := inst.Field("netloc").(*object.String)
			if netloc == nil {
				return &object.Null{}
			}
			userinfo, hasUser, hostinfo := netlocHostinfo(netloc.StringValue())
			switch name {
			case "username", "password":
				if !hasUser {
					return &object.Null{}
				}
				user, pass, hasPass := strings.Cut(userinfo, ":")
				if name == "username" {
					return object.NewString(user)
				}
				if !hasPass {
					return &object.Null{}
				}
				return object.NewString(pass)
			}
			var host, port string
			if _, bracketed, ok := strings.Cut(hostinfo, "["); ok {
				h, rest, _ := strings.Cut(bracketed, "]")
				host = h
				_, port, _ = strings.Cut(rest, ":")
			} else {
				host, port, _ = strings.Cut(hostinfo, ":")
			}
			if name == "hostname" {
				if host == "" {
					return &object.Null{}
				}
				return object.NewString(strings.ToLower(host))
			}
			if port == "" {
				return &object.Null{}
			}
			n, err := strconv.Atoi(port)
			if err != nil || strings.ContainsAny(port, "+-") {
				return errors.NewValueError("Port could not be cast to integer value as %s", object.ReprString(port))
			}
			if n < 0 || n > 65535 {
				return errors.NewValueError("Port out of range 0-65535")
			}
			return object.NewInteger(int64(n))
		},
		HelpText: name + " - from the netloc, as Python computes it",
	}}
}

func init() {
	for _, c := range []*object.Class{ParseResultClass, SplitResultClass, DefragResultClass} {
		c := c
		m := c.Methods
		m["geturl"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.NoKwargs("geturl", kwargs); err != nil {
					return err
				}
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				v := urlResultValues(args[0].(*object.Instance))
				switch c {
				case SplitResultClass:
					return object.NewString(pyURLUnsplit(v[0], v[1], v[2], v[3], v[4]))
				case DefragResultClass:
					if v[1] != "" {
						return object.NewString(v[0] + "#" + v[1])
					}
					return object.NewString(v[0])
				}
				return object.NewString(pyURLUnparse([6]string(v)))
			},
			HelpText: "geturl() - The URL the parts make",
		}
		m["__getitem__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				v := urlResultValues(args[0].(*object.Instance))
				elems := make([]object.Object, len(v))
				for i, s := range v {
					elems[i] = object.NewString(s)
				}
				switch k := args[1].(type) {
				case *object.Integer:
					i := k.IntValue()
					if i < 0 {
						i += int64(len(elems))
					}
					if i < 0 || i >= int64(len(elems)) {
						return errors.NewIndexError("tuple index out of range")
					}
					return elems[i]
				}
				return errors.NewTypeErrorTagged("tuple indices must be integers or slices, not %s", object.PythonTypeName(args[1]))
			},
			HelpText: "__getitem__(i) - The i-th part",
		}
		m["__len__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return object.NewInteger(int64(len(urlResultFields(c))))
			},
			HelpText: "__len__() - The number of parts",
		}
		m["__repr__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				v := urlResultValues(args[0].(*object.Instance))
				parts := make([]string, len(v))
				for i, n := range urlResultFields(c) {
					parts[i] = n + "=" + object.ReprString(v[i])
				}
				return object.NewString(c.Name + "(" + strings.Join(parts, ", ") + ")")
			},
			HelpText: "__repr__() - Name(field='value', ...)",
		}
		m["__eq__"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return errors.NewArgumentError(len(args), 2)
				}
				v := urlResultValues(args[0].(*object.Instance))
				var other []object.Object
				switch o := args[1].(type) {
				case *object.Tuple:
					other = o.Elements
				case *object.Instance:
					if o.Class != c {
						return object.NewBoolean(false)
					}
					return object.NewBoolean(slices.Equal(v, urlResultValues(o)))
				default:
					return object.NewBoolean(false)
				}
				if len(other) != len(v) {
					return object.NewBoolean(false)
				}
				for i, e := range other {
					s, ok := e.(*object.String)
					if !ok || s.StringValue() != v[i] {
						return object.NewBoolean(false)
					}
				}
				return object.NewBoolean(true)
			},
			HelpText: "__eq__(other) - Equal to a result or tuple with the same parts",
		}
		m["_replace"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				names := urlResultFields(c)
				if kwargs.Len() > 0 {
					if err := errors.CheckKwargs("_replace", kwargs, names...); err != nil {
						return err
					}
				}
				v := urlResultValues(args[0].(*object.Instance))
				for i, n := range names {
					if nv := kwargs.Get(n); nv != nil {
						s, ok := nv.(*object.String)
						if !ok {
							return errors.NewTypeErrorTagged("%s must be str", n)
						}
						v[i] = s.StringValue()
					}
				}
				return newURLResult(c, v...)
			},
			HelpText: "_replace(**parts) - A copy with the given parts replaced",
		}
		m["_asdict"] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil {
					return err
				}
				d := object.NewDict()
				v := urlResultValues(args[0].(*object.Instance))
				for i, n := range urlResultFields(c) {
					k := object.NewString(n)
					d.Store(object.DictKey(k), k, object.NewString(v[i]))
				}
				return d
			},
			HelpText: "_asdict() - The parts as a dict",
		}
		if c != DefragResultClass {
			for _, p := range []string{"hostname", "port", "username", "password"} {
				m[p] = urlResultProperty(p)
			}
		}
	}

	fns := URLParseLibrary.Functions()
	fns["urlsplit"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			url, scheme, allow, errObj := urlArgs("urlsplit", args, kwargs)
			if errObj != nil {
				return errObj
			}
			s, errObj := pyURLSplit(url, scheme, allow)
			if errObj != nil {
				return errObj
			}
			return newURLResult(SplitResultClass, s[:]...)
		},
		HelpText: `urlsplit(url, scheme='', allow_fragments=True) - Split a URL into (scheme, netloc, path, query, fragment)

The result unpacks like a tuple and has those attributes, plus hostname,
port, username and password, geturl() and _replace(**parts).`,
	}
	fns["urlparse"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			url, scheme, allow, errObj := urlArgs("urlparse", args, kwargs)
			if errObj != nil {
				return errObj
			}
			p, errObj := pyURLParse(url, scheme, allow)
			if errObj != nil {
				return errObj
			}
			return newURLResult(ParseResultClass, p[:]...)
		},
		HelpText: `urlparse(url, scheme='', allow_fragments=True) - Parse a URL into (scheme, netloc, path, params, query, fragment)

The result unpacks like a tuple and has those attributes, plus hostname,
port, username and password, geturl() and _replace(**parts).`,
	}
	fns["urlunsplit"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("urlunsplit", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			v, errObj := urlComponents(ctx, args[0], 5)
			if errObj != nil {
				return errObj
			}
			return object.NewString(pyURLUnsplit(v[0], v[1], v[2], v[3], v[4]))
		},
		HelpText: "urlunsplit(parts) - The URL of (scheme, netloc, path, query, fragment)",
	}
	fns["urlunparse"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("urlunparse", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if d, ok := args[0].(*object.Dict); ok {
				// Scriptling has long accepted a dict of the parts too.
				var v [6]string
				for i, n := range parseResultFields {
					if pair, ok := d.GetByString(n); ok {
						if s, ok := pair.Value.(*object.String); ok {
							v[i] = s.StringValue()
						}
					}
				}
				return object.NewString(pyURLUnparse(v))
			}
			v, errObj := urlComponents(ctx, args[0], 6)
			if errObj != nil {
				return errObj
			}
			return object.NewString(pyURLUnparse([6]string(v)))
		},
		HelpText: "urlunparse(parts) - The URL of (scheme, netloc, path, params, query, fragment)",
	}
	fns["urljoin"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("urljoin", kwargs, "allow_fragments"); err != nil {
					return err
				}
			}
			if len(args) < 2 || len(args) > 3 {
				return errors.NewTypeErrorTagged("urljoin() takes 2 or 3 arguments (%d given)", len(args))
			}
			base, err := args[0].AsString()
			if err != nil {
				return err
			}
			ref, err := args[1].AsString()
			if err != nil {
				return err
			}
			allow := true
			if len(args) == 3 {
				allow = isTruthy(args[2])
			} else if v := kwargs.Get("allow_fragments"); v != nil {
				allow = isTruthy(v)
			}
			out, errObj := pyURLJoin(base, ref, allow)
			if errObj != nil {
				return errObj
			}
			return object.NewString(out)
		},
		HelpText: "urljoin(base, url, allow_fragments=True) - Resolve url against base, as a browser would",
	}
	fns["urldefrag"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("urldefrag", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			url, err := args[0].AsString()
			if err != nil {
				return err
			}
			if !strings.Contains(url, "#") {
				return newURLResult(DefragResultClass, url, "")
			}
			p, errObj := pyURLParse(url, "", true)
			if errObj != nil {
				return errObj
			}
			frag := p[5]
			p[5] = ""
			return newURLResult(DefragResultClass, pyURLUnparse(p), frag)
		},
		HelpText: "urldefrag(url) - (url without its fragment, fragment)",
	}
}

// createParseResultInstance builds a urlparse() result from its parts.
func createParseResultInstance(scheme, netloc, path, params, query, fragment string) *object.Instance {
	return newURLResult(ParseResultClass, scheme, netloc, path, params, query, fragment)
}
