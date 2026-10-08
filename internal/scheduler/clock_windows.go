package scheduler

import (
	"syscall"
	"time"
	"unsafe"
)

// Go caches time.Local. Query Windows each poll so timezone changes take effect without restarting.
func Now() time.Time {
	var st struct{ Year, Month, Weekday, Day, Hour, Minute, Second, Millis uint16 }
	syscall.NewLazyDLL("kernel32.dll").NewProc("GetLocalTime").Call(uintptr(unsafe.Pointer(&st)))
	now := time.Now()
	wall := time.Date(int(st.Year), time.Month(st.Month), int(st.Day), int(st.Hour), int(st.Minute), int(st.Second), int(st.Millis)*1000000, time.UTC)
	offset := int(wall.Sub(now.UTC()).Round(time.Minute).Seconds())
	return now.In(time.FixedZone("Windows local", offset))
}
