package agentpicker

import (
	"bytes"
	"encoding/binary"
	"errors"
)

func decodeProcessArgs(data []byte) ([]string, error) {
	invalid := errors.New("process arguments unavailable")
	if len(data) < 4 {
		return nil, invalid
	}
	argc := int(binary.LittleEndian.Uint32(data[:4]))
	data = data[4:]
	if argc < 1 || argc > len(data) {
		return nil, invalid
	}
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		return nil, invalid
	}
	data = bytes.TrimLeft(data[end+1:], "\x00")
	args := make([]string, 0, argc)
	for i := 0; i < argc; i++ {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, invalid
		}
		args = append(args, string(data[:end]))
		data = data[end+1:]
	}
	return args, nil
}
