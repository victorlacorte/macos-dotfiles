//go:build !darwin && !linux

package agentpicker

import "errors"

func processArgs(pid int) ([]string, error) {
	return nil, errors.New("process arguments are unsupported on this platform")
}
