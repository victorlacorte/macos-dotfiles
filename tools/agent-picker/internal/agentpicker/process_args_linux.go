package agentpicker

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
)

func processArgs(pid int) ([]string, error) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || data[len(data)-1] != 0 {
		return nil, fmt.Errorf("process arguments unavailable")
	}
	parts := bytes.Split(data[:len(data)-1], []byte{0})
	args := make([]string, len(parts))
	for i, part := range parts {
		args[i] = string(part)
	}
	return args, nil
}
