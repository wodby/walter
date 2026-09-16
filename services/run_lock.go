package services

import (
	"fmt"
	"os"
	"syscall"
)

// AcquireRunLock holds a non-blocking process lock for the complete service run.
// The sidecar must remain in place: unlinking it would allow a second lock inode.
func AcquireRunLock(statePath string) (func(), error) {
	f, err := os.OpenFile(statePath+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("open service lock: %w", err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("service lock must be a regular file")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("service run is locked: %w", err)
	}
	return func() { f.Close() }, nil
}
