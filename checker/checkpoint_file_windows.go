package checker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Go's os.Open omits FILE_SHARE_DELETE on Windows, preventing replacement
// while a reader has the old checkpoint open. A shared-delete handle keeps the
// old contents readable even after another process replaces the pathname.
func checkpointOpen(path string) (*os.File, error) {
	return checkpointOpenContext(context.Background(), path)
}

func checkpointOpenContext(ctx context.Context, path string) (*os.File, error) {
	openPath := checkpointWindowsPath(path)
	name, err := syscall.UTF16PtrFromString(openPath)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var handle syscall.Handle
	err = checkpointRetry(ctx, func() error {
		var openErr error
		handle, openErr = syscall.CreateFile(name, syscall.GENERIC_READ,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		return openErr
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}

func checkpointWindowsPath(path string) string {
	openPath := path
	if abs, err := filepath.Abs(path); err == nil && len(abs) >= 248 && !strings.HasPrefix(abs, `\\?\`) && !strings.HasPrefix(abs, `\\.\`) {
		if strings.HasPrefix(abs, `\\`) {
			openPath = `\\?\UNC\` + abs[2:]
		} else {
			openPath = `\\?\` + abs
		}
	}
	return openPath
}

func checkpointReplaceContext(ctx context.Context, oldpath, newpath string) error {
	return checkpointRetry(ctx, func() error { return checkpointReplaceWindows(oldpath, newpath) })
}

var checkpointSetFileInformation = syscall.NewLazyDLL("kernel32.dll").NewProc("SetFileInformationByHandle")

// The temporary file is created in the target directory. A single POSIX-style
// rename handles both initial creation and replacement, including concurrent
// writers, while readers with delete sharing retain their old handles.
func checkpointReplaceWindows(oldpath, newpath string) error {
	linkError := func(err error) error { return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err} }
	oldname, err := syscall.UTF16PtrFromString(checkpointWindowsPath(oldpath))
	if err != nil {
		return linkError(err)
	}
	name, err := syscall.UTF16FromString(checkpointWindowsPath(newpath))
	if err != nil {
		return linkError(err)
	}
	const deleteAccess = 0x10000
	h, err := syscall.CreateFile(oldname, deleteAccess, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return linkError(err)
	}
	defer syscall.CloseHandle(h)

	// FILE_RENAME_INFO has a variable-length UTF-16 name and pointer alignment.
	type renameInfo struct {
		flags  uint32
		root   syscall.Handle
		length uint32
		name   [1]uint16
	}
	const fileRenameInfoEx = 22
	const replaceAndPOSIX = 0x1 | 0x2
	nameBytes := (len(name) - 1) * 2
	// Keep the UTF-16 terminator in the buffer even though FileNameLength
	// excludes it, as required by the documented FileName contract.
	size := max(int(unsafe.Sizeof(renameInfo{})), int(unsafe.Offsetof(renameInfo{}.name))+len(name)*2)
	storage := make([]uintptr, (size+int(unsafe.Sizeof(uintptr(0)))-1)/int(unsafe.Sizeof(uintptr(0))))
	info := (*renameInfo)(unsafe.Pointer(&storage[0]))
	info.flags = replaceAndPOSIX
	info.length = uint32(nameBytes)
	copy(unsafe.Slice(&info.name[0], len(name)), name)
	r, _, callErr := checkpointSetFileInformation.Call(uintptr(h), fileRenameInfoEx, uintptr(unsafe.Pointer(info)), uintptr(size))
	if r == 0 {
		return linkError(callErr)
	}
	return nil
}

// Other processes, including antivirus scanners and concurrent replacements,
// can briefly hold handles that prevent opening or replacing a file. Bound the
// wait so a permanent access error is reported instead of hanging indefinitely.
func checkpointRetry(ctx context.Context, operation func() error) error {
	return checkpointRetryUntil(ctx, time.Now().Add(2*time.Second), operation)
}

func checkpointRetryUntil(ctx context.Context, deadline time.Time, operation func() error) error {
	var lastErr error
	for {
		if err := checkpointRetryStopped(ctx, deadline, lastErr); err != nil {
			return err
		}
		err := operation()
		if err == nil {
			return nil
		}
		err = checkpointContextError(ctx, err)
		if !checkpointRetryable(err) || !time.Now().Before(deadline) {
			return err
		}
		lastErr = err
		if err := checkpointRetryWait(ctx); err != nil {
			return err
		}
	}
}

func checkpointRetryStopped(ctx context.Context, deadline time.Time, lastErr error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if lastErr != nil && !time.Now().Before(deadline) {
		return lastErr
	}
	return nil
}

func checkpointRetryable(err error) bool {
	const errorSharingViolation = syscall.Errno(32)
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED) || errors.Is(err, errorSharingViolation)
}

func checkpointRetryWait(ctx context.Context) error {
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
