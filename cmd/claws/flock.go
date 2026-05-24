package main

import (
	"os"
	"path/filepath"
	"syscall"
)

// withFileLock acquires an exclusive lock on lockPath, runs fn, then releases.
// The lock file is created if it doesn't exist.
func withFileLock(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	return fn()
}

// withRegistryLock locks the port registry for exclusive access.
func withRegistryLock(paths Paths, fn func() error) error {
	lockPath := filepath.Join(paths.Root, ".port-registry.lock")
	return withFileLock(lockPath, fn)
}

// withGroupLock locks a group's config for exclusive access.
func withGroupLock(groupDir string, fn func() error) error {
	lockPath := filepath.Join(groupDir, ".group.lock")
	return withFileLock(lockPath, fn)
}

// withInstanceLock locks an instance's env file for exclusive access.
func withInstanceLock(instanceDir string, fn func() error) error {
	lockPath := filepath.Join(instanceDir, ".instance.lock")
	return withFileLock(lockPath, fn)
}
