package container

import (
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// checkRun applies the registration's restrictions to a run() request
// before it reaches the runtime. A container is outside the interpreter, so
// what it is given is checked here: a bind mount must not hand it a host path
// outside the jail, a privileged container (which can reach every host disk)
// is refused when DenyPrivileged is set, and host networking is refused under
// a network policy. Denials are uncatchable PermissionErrors, as other jail
// denials are. A nil policy (no restrictions) allows everything.
func (p *endpointPolicy) checkRun(opts RunOptions) object.Object {
	if p == nil {
		return nil
	}
	if opts.Privileged && p.denyPrivileged {
		return errors.NewPermissionError("run: privileged containers are not allowed")
	}
	if p.guard != nil && strings.EqualFold(opts.Network, "host") {
		return errors.NewPermissionError("run: network \"host\" is not allowed under a network policy")
	}
	if p.paths.AllowedPaths != nil {
		for _, spec := range opts.Volumes {
			if src, ok := bindSource(spec); ok && !p.paths.IsPathAllowed(src) {
				return errors.NewPermissionError("run: volume %q mounts host path %q, which is outside allowed directories", spec, src)
			}
		}
	}
	return nil
}

// bindSource returns the host path a volume spec binds, and whether it binds
// one. Specs are "source:target[:options]" (or the --mount style
// "type=bind,source=...,target=..."); a source with no path separator and no
// leading "." or "~" is a named volume, and a spec with no source ("/data")
// is an anonymous volume. Anything that could name a host path is treated as
// one, so an unusual spelling is checked rather than let through.
func bindSource(spec string) (string, bool) {
	if strings.Contains(spec, "=") && !strings.HasPrefix(spec, "/") {
		for _, field := range strings.Split(spec, ",") {
			key, value, _ := strings.Cut(field, "=")
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "source", "src":
				return value, isHostPath(value)
			}
		}
		return "", false
	}
	src, _, hasTarget := strings.Cut(spec, ":")
	if !hasTarget {
		return "", false
	}
	return src, isHostPath(src)
}

func isHostPath(src string) bool {
	return strings.ContainsAny(src, `/\`) || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "~")
}
