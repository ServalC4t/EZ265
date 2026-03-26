//go:build !windows

package registry

func IsRegistered() bool { return false }

func Register() error { return nil }

func Unregister() error { return nil }
