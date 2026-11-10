package fetch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/provision/become"
	"github.com/paularlott/scriptling/object"
)

// Writing what was fetched runs here, or as root in the become helper.
func init() {
	become.Handle("fetch.provided", opProvided)
	become.Handle("fetch.write", opWrite)
	become.Handle("fetch.unzip", opUnzip)
}

type providedArgs struct {
	Paths []string
}

type writeArgs struct {
	Data    []byte
	Dest    string
	Mode    os.FileMode
	DirMode os.FileMode
	Owner   become.Owner
}

// opProvided reports whether every path exists. Only a path that can be
// stat'ed exists; an unreadable one (say, permission denied) does not count.
func opProvided(_ context.Context, paths fssecurity.Config, a providedArgs) (bool, error) {
	for _, p := range a.Paths {
		if err := paths.CheckPath(p); err != nil {
			return false, err
		}
		if _, err := os.Stat(p); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func opWrite(_ context.Context, paths fssecurity.Config, a writeArgs) (fetchResult, error) {
	if err := paths.CheckPath(a.Dest); err != nil {
		return fetchResult{}, err
	}
	r, err := writeFetchedFile(a.Data, a.Dest, a.Mode, a.DirMode)
	if err != nil {
		return r, err
	}
	changed, err := a.Owner.Apply(paths, a.Dest)
	if err != nil {
		return r, err
	}
	if changed && r.Status == StatusUnchanged {
		r.Status = StatusUpdated
	}
	return r, nil
}

func opUnzip(_ context.Context, paths fssecurity.Config, a writeArgs) (fetchResult, error) {
	if err := paths.CheckPath(a.Dest); err != nil {
		return fetchResult{}, err
	}
	r, err := unpackZipBytes(a.Data, a.Dest, a.Mode, a.DirMode, paths.CheckPath)
	if err != nil || !a.Owner.Given() {
		return r, err
	}
	// The owner goes to dest, everything unpacked and the directories
	// between them; nothing else in dest is touched.
	dest, err := filepath.Abs(a.Dest)
	if err != nil {
		return r, err
	}
	owned := map[string]bool{}
	changed := false
	own := func(p string) error {
		if owned[p] {
			return nil
		}
		owned[p] = true
		c, err := a.Owner.Apply(paths, p)
		changed = changed || c
		return err
	}
	if err := own(dest); err != nil {
		return r, err
	}
	for _, f := range r.Files {
		for d := filepath.Dir(f); len(d) > len(dest); d = filepath.Dir(d) {
			if err := own(d); err != nil {
				return r, err
			}
		}
		if err := own(f); err != nil {
			return r, err
		}
	}
	if changed && r.Status == StatusUnchanged {
		r.Status = StatusUpdated
	}
	return r, nil
}

// ownerArgs reads the owner and group arguments: a name or a number each.
func ownerArgs(kwargs object.Kwargs) (become.Owner, object.Object) {
	var owner become.Owner
	for _, k := range []struct {
		arg string
		to  *string
	}{{"owner", &owner.User}, {"group", &owner.Group}} {
		switch v := kwargs.Get(k.arg).(type) {
		case nil, *object.Null:
		case *object.String:
			*k.to, _ = v.AsString()
		case *object.Integer:
			if v.IntValue() < 0 {
				return owner, &object.Error{Message: fmt.Sprintf("file: %s must not be negative", k.arg)}
			}
			*k.to = strconv.FormatInt(v.IntValue(), 10)
		default:
			return owner, &object.Error{Message: fmt.Sprintf("file: %s must be a name or a number", k.arg)}
		}
	}
	return owner, nil
}
