package main

import (
	"bytes"

	"github.com/abustany/lcp-decrypt/pkg/lcp"
)

var handles = map[*byte][]byte{}

func main() {
}

//export newBytes
func newBytes(size int) *byte {
	if size == 0 {
		return nil
	}
	return newHandle(make([]byte, size))
}

func newHandle(b []byte) *byte {
	if len(b) == 0 {
		return nil
	}
	handles[&b[0]] = b
	return &b[0]
}

//export freeBytes
func freeBytes(ptr *byte) {
	delete(handles, ptr)
}

//export bytesSize
func bytesSize(ptr *byte) int {
	return len(handles[ptr])
}

//export decrypt
func decrypt(inPtr *byte, userKeyHexPtr *byte) *byte {
	var out bytes.Buffer
	inputData := handles[inPtr]

	if err := lcp.Decrypt(&out, bytes.NewReader(handles[inPtr]), int64(len(inputData)), string(handles[userKeyHexPtr])); err != nil {
		panic(err.Error())
	}

	return newHandle(out.Bytes())
}
