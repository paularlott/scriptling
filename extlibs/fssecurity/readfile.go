package fssecurity

import (
	"context"
	"fmt"
	"os"
	"syscall"

	scripterrors "github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// ReadFile reads path for a script after checking it against cfg. A path
// outside the allowed paths is a PermissionError; a read failure is the
// matching OSError-family exception (see PathErrorException); a file larger
// than the call's limits allow is the limit error from
// object.ReadFileLimited. The read runs through object.RunBlocking so it
// counts as blocking time for the call.
func ReadFile(ctx context.Context, cfg Config, path string) ([]byte, object.Object) {
	if !cfg.IsPathAllowed(path) {
		return nil, scripterrors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	var content []byte
	var limitErr object.Object
	var err error
	object.RunBlocking(ctx, func() { content, limitErr, err = object.ReadFileLimited(path) })
	if err != nil {
		return nil, PathErrorException("cannot read file", err)
	}
	if limitErr != nil {
		return nil, limitErr
	}
	return content, nil
}

// PathErrorException converts a Go filesystem error into the matching
// Python OSError-family exception so `except FileNotFoundError:` and friends
// catch file-operation failures: ENOENT→FileNotFoundError, EEXIST→
// FileExistsError, EACCES/EPERM→PermissionError, EISDIR→IsADirectoryError,
// ENOTDIR→NotADirectoryError, anything else→OSError. what prefixes the
// message ("cannot read file: ...").
func PathErrorException(what string, err error) object.Object {
	excType := object.ExceptionTypeOSError
	errno, ok := err.(syscall.Errno)
	if !ok {
		if pe, isPE := err.(*os.PathError); isPE {
			errno, ok = pe.Err.(syscall.Errno)
		}
	}
	if ok {
		switch errno {
		case syscall.ENOENT:
			excType = "FileNotFoundError"
		case syscall.EEXIST:
			excType = "FileExistsError"
		case syscall.EACCES, syscall.EPERM:
			excType = object.ExceptionTypePermissionError
		case syscall.EISDIR:
			excType = "IsADirectoryError"
		case syscall.ENOTDIR:
			excType = "NotADirectoryError"
		}
	}
	return &object.Exception{
		Message:       fmt.Sprintf("%s: %s", what, err.Error()),
		ExceptionType: excType,
		Raised:        true,
	}
}
