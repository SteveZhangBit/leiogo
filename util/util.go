package util

import (
	"crypto/md5"
	"fmt"
	"io"
)

func MD5Hash(input string) string {
	h := md5.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}
