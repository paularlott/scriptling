// Package packages is scriptling.provision.package: install and remove
// system packages, skipping those already as they should be.
package packages

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	"github.com/paularlott/scriptling/extlibs/provision/command"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.package"
	LibraryDesc = "Install and remove system packages with apt, dnf, yum, apk or Homebrew, skipping those already in place"

	StatusCreated   = "created"
	StatusUnchanged = "unchanged"
	StatusRemoved   = "removed"
	StatusAbsent    = "absent"
)

// manager is a package manager: how to ask it about a package, install,
// remove and refresh its package lists.
type manager struct {
	name      string
	program   string // its presence picks it
	root      bool   // installing needs root
	env       []string
	installed func(name string) []string // a command that exits 0 when name is installed
	install   []string
	remove    []string
	update    []string
}

// managers, in the order they are looked for.
var managers = []manager{
	{
		name: "apt", program: "apt-get", root: true,
		env: []string{"DEBIAN_FRONTEND=noninteractive"},
		installed: func(n string) []string {
			return []string{"dpkg-query", "-W", "-f=${db:Status-Abbrev}", n}
		},
		// Waits for another apt (unattended-upgrades on a new machine, say)
		// rather than failing at its lock.
		install: []string{"apt-get", "install", "-y", "-q", "-o", "DPkg::Lock::Timeout=300"},
		remove:  []string{"apt-get", "remove", "-y", "-q", "-o", "DPkg::Lock::Timeout=300"},
		update:  []string{"apt-get", "update", "-q", "-o", "DPkg::Lock::Timeout=300"},
	},
	{
		name: "dnf", program: "dnf", root: true,
		installed: func(n string) []string { return []string{"rpm", "-q", "--quiet", n} },
		install:   []string{"dnf", "install", "-y", "-q"},
		remove:    []string{"dnf", "remove", "-y", "-q"},
		update:    []string{"dnf", "makecache", "-q"},
	},
	{
		name: "yum", program: "yum", root: true,
		installed: func(n string) []string { return []string{"rpm", "-q", "--quiet", n} },
		install:   []string{"yum", "install", "-y", "-q"},
		remove:    []string{"yum", "remove", "-y", "-q"},
		update:    []string{"yum", "makecache", "-q"},
	},
	{
		name: "apk", program: "apk", root: true,
		installed: func(n string) []string { return []string{"apk", "info", "-e", "-q", n} },
		install:   []string{"apk", "add", "-q", "--no-progress"},
		remove:    []string{"apk", "del", "-q", "--no-progress"},
		update:    []string{"apk", "update", "-q"},
	},
	{
		// Homebrew refuses to run as root, and updates itself before each
		// install unless told not to: here it is updated only when asked, or
		// when an install fails.
		name: "brew", program: "brew", root: false,
		env:       []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ENV_HINTS=1"},
		installed: func(n string) []string { return []string{"brew", "list", "--versions", n} },
		install:   []string{"brew", "install", "--quiet"},
		remove:    []string{"brew", "uninstall", "--quiet"},
		update:    []string{"brew", "update", "--quiet"},
	},
}

// lookPath finds a program; tests replace it.
var lookPath = exec.LookPath

func detect(name string) (*manager, error) {
	for i := range managers {
		m := &managers[i]
		if name != "" {
			if m.name == name {
				return m, nil
			}
			continue
		}
		if _, err := lookPath(m.program); err == nil {
			return m, nil
		}
	}
	if name != "" {
		return nil, fmt.Errorf("no package manager %q (have apt, dnf, yum, apk and brew)", name)
	}
	return nil, fmt.Errorf("found no package manager (looked for apt-get, dnf, yum, apk and brew)")
}

type libraryInstance struct {
	become *become.Become
}

// Option configures the library.
type Option func(*libraryInstance)

// WithBecome lets the library install packages as root, through b. Package
// managers that need root use it on their own; Homebrew never does.
func WithBecome(b *become.Become) Option {
	return func(i *libraryInstance) { i.become = b }
}

// Register registers the library with running as root off: it can install
// packages only when the process is root (or with Homebrew).
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	RegisterConfigured(registrar)
}

// RegisterConfigured registers the library with its options.
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, opts ...Option) {
	inst := &libraryInstance{}
	for _, o := range opts {
		o(inst)
	}
	registrar.RegisterLibrary(buildLibrary(inst))
}

// session runs one call's commands.
type session struct {
	ctx     context.Context
	inst    *libraryInstance
	m       *manager
	updated bool
}

func (s *session) run(asRoot bool, argv []string) (become.RunResult, error) {
	var res become.RunResult
	var err error
	object.RunBlocking(s.ctx, func() {
		res, err = become.Run(s.ctx, s.inst.become, asRoot && s.m.root, become.RunArgs{Argv: argv, Env: s.m.env})
	})
	return res, err
}

func (s *session) isInstalled(name string) (bool, error) {
	res, err := s.run(false, s.m.installed(name))
	if err != nil {
		return false, err
	}
	if s.m.name == "apt" { // "ii": installed; "rc": removed, its configuration left
		return res.Code == 0 && strings.HasPrefix(string(res.Stdout), "ii"), nil
	}
	return res.Code == 0, nil
}

func (s *session) refresh() error {
	s.updated = true
	res, err := s.run(true, s.m.update)
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", command.Failure(s.m.update, res))
	}
	return nil
}

// change installs or removes names. With retry, a failure is tried once more
// after refreshing the package lists, which a new machine (or one whose lists
// are stale) needs to install anything.
func (s *session) change(base []string, names []string, retry bool) error {
	argv := append(append([]string{}, base...), names...)
	res, err := s.run(true, argv)
	if err != nil {
		return err
	}
	if res.Code == 0 {
		return nil
	}
	if s.updated || !retry {
		return fmt.Errorf("%s", command.Failure(argv, res))
	}
	if err := s.refresh(); err != nil {
		return err
	}
	res, err = s.run(true, argv)
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("%s", command.Failure(argv, res))
	}
	return nil
}

// names reads a package name or a list of them.
func names(fn string, arg object.Object) ([]string, object.Object) {
	var out []string
	add := func(o object.Object) object.Object {
		s, ok := o.(*object.String)
		if !ok {
			return errors.NewTypeErrorTagged("%s: a package name must be a string, not %s", fn, o.Inspect())
		}
		v, _ := s.AsString()
		v = strings.TrimSpace(v)
		if v == "" || strings.HasPrefix(v, "-") {
			return errors.NewValueError("%s: %q is not a package name", fn, v)
		}
		out = append(out, v)
		return nil
	}
	switch a := arg.(type) {
	case *object.List:
		for _, el := range a.Elements {
			if e := add(el); e != nil {
				return nil, e
			}
		}
	default:
		if e := add(arg); e != nil {
			return nil, e
		}
	}
	if len(out) == 0 {
		return nil, errors.NewValueError("%s: no packages named", fn)
	}
	return out, nil
}

func (inst *libraryInstance) start(ctx context.Context, fn string, kwargs object.Kwargs) (*session, object.Object) {
	want, errObj := kwargs.GetString("manager", "")
	if errObj != nil {
		return nil, errObj
	}
	m, err := detect(want)
	if err != nil {
		return nil, &object.Error{Message: fn + ": " + err.Error()}
	}
	return &session{ctx: ctx, inst: inst, m: m}, nil
}

func buildLibrary(inst *libraryInstance) *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"ensure": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("ensure expected 1 argument, got %d", len(args))}
				}
				pkgs, errObj := names("ensure", args[0])
				if errObj != nil {
					return errObj
				}
				update, errObj := kwargs.GetBool("update", false)
				if errObj != nil {
					return errObj
				}
				s, errObj := inst.start(ctx, "ensure", kwargs)
				if errObj != nil {
					return errObj
				}
				var missing []string
				for _, p := range pkgs {
					ok, err := s.isInstalled(p)
					if err != nil {
						return command.PermissionError("ensure", err)
					}
					if !ok {
						missing = append(missing, p)
					}
				}
				if len(missing) == 0 {
					return object.NewString(StatusUnchanged)
				}
				if update {
					if err := s.refresh(); err != nil {
						return command.PermissionError("ensure", err)
					}
				}
				if err := s.change(s.m.install, missing, true); err != nil {
					return command.PermissionError("ensure", err)
				}
				return object.NewString(StatusCreated)
			},
			HelpText: `ensure(packages, update=False, manager=None) - Ensure packages are installed

Installs those of packages that are not installed; when all of them are, the
package manager is not run at all. If an install fails, the package lists are
refreshed and it is tried once more, as a new machine needs.

apt, dnf, yum and apk install as root: a script running as a user does so
through sudo, asked once (without a password where sudo allows that). The
scriptling CLI refuses it with --deny-become. Homebrew installs as the user.

Parameters:
  packages (str or list[str]): A package name, or a list of them
  update (bool): Refresh the package lists before installing (default False)
  manager (str): "apt", "dnf", "yum", "apk" or "brew". Default: the first of these found

Returns:
  str: package.CREATED if anything was installed, package.UNCHANGED if all were already

Example:
  import scriptling.provision.package as package
  if package.ensure(["nginx", "curl"]) == package.CREATED:
      print("installed")`,
		},
		"absent": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("absent expected 1 argument, got %d", len(args))}
				}
				pkgs, errObj := names("absent", args[0])
				if errObj != nil {
					return errObj
				}
				s, errObj := inst.start(ctx, "absent", kwargs)
				if errObj != nil {
					return errObj
				}
				var present []string
				for _, p := range pkgs {
					ok, err := s.isInstalled(p)
					if err != nil {
						return command.PermissionError("absent", err)
					}
					if ok {
						present = append(present, p)
					}
				}
				if len(present) == 0 {
					return object.NewString(StatusAbsent)
				}
				if err := s.change(s.m.remove, present, false); err != nil {
					return command.PermissionError("absent", err)
				}
				return object.NewString(StatusRemoved)
			},
			HelpText: `absent(packages, manager=None) - Ensure packages are not installed

Removes those of packages that are installed; when none is, the package
manager is not run.

Parameters:
  packages (str or list[str]): A package name, or a list of them
  manager (str): As for ensure()

Returns:
  str: package.REMOVED if anything was removed, package.ABSENT if none was installed

Example:
  import scriptling.provision.package as package
  package.absent("telnet")`,
		},
		"installed": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("installed expected 1 argument, got %d", len(args))}
				}
				pkgs, errObj := names("installed", args[0])
				if errObj != nil {
					return errObj
				}
				s, errObj := inst.start(ctx, "installed", kwargs)
				if errObj != nil {
					return errObj
				}
				for _, p := range pkgs {
					ok, err := s.isInstalled(p)
					if err != nil {
						return &object.Error{Message: "installed: " + err.Error()}
					}
					if !ok {
						return object.NewBoolean(false)
					}
				}
				return object.NewBoolean(true)
			},
			HelpText: `installed(packages, manager=None) - Whether packages are installed

Parameters:
  packages (str or list[str]): A package name, or a list of them
  manager (str): As for ensure()

Returns:
  bool: True when every one is installed`,
		},
		"manager": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 0); err != nil {
					return err
				}
				m, err := detect("")
				if err != nil {
					return &object.Null{}
				}
				return object.NewString(m.name)
			},
			HelpText: `manager() - The package manager ensure() would use

Returns:
  str or None: "apt", "dnf", "yum", "apk" or "brew"; None when there is none`,
		},
	}, map[string]object.Object{
		"CREATED":   object.NewString(StatusCreated),
		"UNCHANGED": object.NewString(StatusUnchanged),
		"REMOVED":   object.NewString(StatusRemoved),
		"ABSENT":    object.NewString(StatusAbsent),
	}, LibraryDesc)
}
