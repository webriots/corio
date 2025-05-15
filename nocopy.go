package corio

// noCopy is a type that prevents copying of values that embed it. It
// implements sync.Locker to provide a standard way to detect improper
// copying. This is similar to sync.Mutex's embedded noCopy field.
type noCopy struct{}

// Lock is a no-op implementation of sync.Locker.Lock.
func (*noCopy) Lock() {}

// Unlock is a no-op implementation of sync.Locker.Unlock.
func (*noCopy) Unlock() {}
