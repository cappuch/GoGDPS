package gdlib

import (
	"strconv"
	"time"
)

// FormatGDDate matches PHP date("d-m-Y G-i", $timestamp).
func FormatGDDate(ts int64) string {
	t := time.Unix(ts, 0)
	return t.Format("02-01-2006 ") + strconv.Itoa(t.Hour()) + "-" + t.Format("04")
}
