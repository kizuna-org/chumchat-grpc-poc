package main

import (
	"bytes"
	"encoding/binary"
)

func Int16ToBytesBinary(ints []int16, order binary.ByteOrder) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, order, ints); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func BytesToInt16Binary(data []byte, order binary.ByteOrder) ([]int16, error) {
	reader := bytes.NewReader(data)
	int16Slice := make([]int16, len(data)/2)
	if err := binary.Read(reader, order, int16Slice); err != nil {
		return nil, err
	}
	return int16Slice, nil
}
