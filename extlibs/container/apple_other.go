//go:build !darwin

package container

import "fmt"

func newAppleDriver() (ContainerDriver, error) {
	return nil, fmt.Errorf("apple containers are only available on macOS")
}
