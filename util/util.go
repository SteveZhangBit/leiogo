package util

import (
	"crypto/md5"
	"fmt"
	"io"
	"time"
)

func MD5Hash(input string) string {
	h := md5.New()
	io.WriteString(h, input)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func FormatDuration(delta time.Duration) string {
	millis := delta / time.Millisecond % 1000
	seconds := delta / time.Second % 60
	minutes := delta / time.Minute % 60
	hours := delta / time.Hour

	if hours > 0 {
		return fmt.Sprintf("%dh:%dm:%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm:%ds", minutes, seconds)
	} else if seconds > 0 {
		return fmt.Sprintf("%ds:%dms", seconds, millis)
	} else {
		return fmt.Sprintf("%dms", millis)
	}
}
