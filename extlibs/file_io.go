package extlibs

import (
	"os"
	"path/filepath"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func normalizeFileIOAllowedPaths(config fssecurity.Config) fssecurity.Config {
	if config.AllowedPaths == nil {
		return config
	}

	normalizedPaths := make([]string, 0, len(config.AllowedPaths))
	for _, p := range config.AllowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}
	config.AllowedPaths = normalizedPaths
	return config
}

func parseFileMode(args []object.Object, kwargs object.Kwargs, index int, defaultMode os.FileMode) (os.FileMode, object.Object) {
	mode := int64(defaultMode)
	if len(args) > index {
		if kwargs.Has("mode") {
			return 0, errors.NewError("mode specified both positionally and by keyword")
		}
		var err object.Object
		mode, err = args[index].AsInt()
		if err != nil {
			return 0, errors.NewTypeError("INTEGER", args[index].Type().String())
		}
	} else if val := kwargs.Get("mode"); val != nil {
		var err object.Object
		mode, err = val.AsInt()
		if err != nil {
			return 0, errors.NewTypeError("INTEGER", val.Type().String())
		}
	}
	if mode < 0 {
		return 0, errors.NewError("mode must be non-negative")
	}
	return os.FileMode(mode), nil
}

func checkPathSecurity(config fssecurity.Config, path string) object.Object {
	if !config.IsPathAllowed(path) {
		return errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	return nil
}

func readFileBytes(config fssecurity.Config, path string) ([]byte, object.Object) {
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewError("cannot read file: %s", err.Error())
	}
	return content, nil
}

func writeFileBytes(config fssecurity.Config, path string, data []byte, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return errors.NewError("cannot write file: %s", err.Error())
	}
	return &object.Null{}
}

func appendFileBytes(config fssecurity.Config, path string, data []byte, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return errors.NewError("cannot open file for append: %s", err.Error())
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return errors.NewError("cannot append to file: %s", err.Error())
	}
	return &object.Null{}
}

func readFileBytesAt(config fssecurity.Config, path string, offset, length, maxLength int64) ([]byte, object.Object) {
	if offset < 0 {
		return nil, errors.NewError("read_bytes: offset must be non-negative")
	}
	if length < 0 {
		return nil, errors.NewError("read_bytes: length must be non-negative")
	}
	if maxLength > 0 && length > maxLength {
		return nil, errors.NewError("read_bytes: length exceeds maximum of %d bytes", maxLength)
	}
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, errors.NewError("read_bytes: cannot open file: %s", err.Error())
	}
	defer file.Close()

	buf := make([]byte, length)
	n, err := file.ReadAt(buf, offset)
	if err != nil && n == 0 {
		return nil, errors.NewError("read_bytes: cannot read file: %s", err.Error())
	}
	return buf[:n], nil
}

func writeFileBytesAt(config fssecurity.Config, path string, offset int64, data []byte, mode os.FileMode) object.Object {
	if offset < 0 {
		return errors.NewError("write_bytes: offset must be non-negative")
	}
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, mode)
	if err != nil {
		return errors.NewError("write_bytes: cannot open file: %s", err.Error())
	}
	defer file.Close()

	if _, err := file.WriteAt(data, offset); err != nil {
		return errors.NewError("write_bytes: cannot write to file: %s", err.Error())
	}
	return &object.Null{}
}

func chmodPath(config fssecurity.Config, path string, mode os.FileMode) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return errors.NewError("cannot change mode: %s", err.Error())
	}
	return &object.Null{}
}

func removePath(config fssecurity.Config, path string, target string, missingOk bool) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if missingOk && os.IsNotExist(err) {
			return &object.Null{}
		}
		return errors.NewError("cannot remove %s: %s", target, err.Error())
	}
	return &object.Null{}
}

func renamePath(config fssecurity.Config, oldPath string, newPath string) object.Object {
	if err := checkPathSecurity(config, oldPath); err != nil {
		return err
	}
	if err := checkPathSecurity(config, newPath); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return errors.NewError("cannot rename: %s", err.Error())
	}
	return &object.Null{}
}

func statPath(config fssecurity.Config, path string, action string) (os.FileInfo, object.Object) {
	if err := checkPathSecurity(config, path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if action == "" {
			return nil, nil
		}
		return nil, errors.NewError("%s: %s", action, err.Error())
	}
	return info, nil
}

func mkdirPath(config fssecurity.Config, path string, mode os.FileMode, parents bool, existOk bool) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}

	var err error
	if parents {
		if !existOk {
			if _, statErr := os.Stat(path); statErr == nil {
				return errors.NewError("cannot create directory: file exists")
			}
		}
		err = os.MkdirAll(path, mode)
	} else {
		err = os.Mkdir(path, mode)
		if existOk && os.IsExist(err) {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				return &object.Null{}
			}
		}
	}
	if err != nil {
		return errors.NewError("cannot create directory: %s", err.Error())
	}
	return &object.Null{}
}

func removeDirs(config fssecurity.Config, path string) object.Object {
	if err := checkPathSecurity(config, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return errors.NewError("cannot remove directory: %s", err.Error())
	}

	parent := filepath.Dir(filepath.Clean(path))
	for parent != "." && parent != string(os.PathSeparator) {
		if isAllowedRoot(config, parent) {
			break
		}
		if !config.IsPathAllowed(parent) {
			break
		}
		if err := os.Remove(parent); err != nil {
			break
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}

	return &object.Null{}
}

func isAllowedRoot(config fssecurity.Config, path string) bool {
	if config.AllowedPaths == nil {
		return false
	}
	cleanPath := filepath.Clean(path)
	for _, allowedPath := range config.AllowedPaths {
		if cleanPath == filepath.Clean(allowedPath) {
			return true
		}
	}
	return false
}
