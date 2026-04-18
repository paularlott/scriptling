package main

import "errors"

type exitCodeError struct {
	code int
	err  error
}

func (e exitCodeError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e exitCodeError) Unwrap() error {
	return e.err
}

func getExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var exitErr exitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.code, true
	}
	return 0, false
}
