package object

import (
	"fmt"
	"reflect"
	"runtime"
)

// SetGCReleaseHook installs hook as a best-effort callback for when target is
// released by Go's garbage collector.
//
// The target must be a non-nil pointer. The hook must not retain target, either
// directly or through captured values, or the object will not become
// unreachable. Finalizers are not prompt and may never run before process exit,
// so callers should use explicit cleanup for resources that need deterministic
// release.
func SetGCReleaseHook(target any, hook func()) error {
	if hook == nil {
		return fmt.Errorf("gc release hook must not be nil")
	}
	targetValue, targetType, err := gcReleaseTarget(target)
	if err != nil {
		return err
	}

	finalizerType := reflect.FuncOf([]reflect.Type{targetType}, nil, false)
	finalizer := reflect.MakeFunc(finalizerType, func([]reflect.Value) []reflect.Value {
		hook()
		return nil
	})

	runtime.SetFinalizer(targetValue.Interface(), finalizer.Interface())
	return nil
}

// ClearGCReleaseHook removes a release hook previously installed with
// SetGCReleaseHook. It is safe to call when no hook is installed.
func ClearGCReleaseHook(target any) error {
	targetValue, _, err := gcReleaseTarget(target)
	if err != nil {
		return err
	}
	runtime.SetFinalizer(targetValue.Interface(), nil)
	return nil
}

func gcReleaseTarget(target any) (reflect.Value, reflect.Type, error) {
	if target == nil {
		return reflect.Value{}, nil, fmt.Errorf("gc release hook target must not be nil")
	}
	targetValue := reflect.ValueOf(target)
	targetType := targetValue.Type()
	if targetType.Kind() != reflect.Ptr {
		return reflect.Value{}, nil, fmt.Errorf("gc release hook target must be a pointer, got %s", targetType)
	}
	if targetValue.IsNil() {
		return reflect.Value{}, nil, fmt.Errorf("gc release hook target must not be nil")
	}
	return targetValue, targetType, nil
}
