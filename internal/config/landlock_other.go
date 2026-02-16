//go:build !linux

package config

func isLandlockAvailable() bool {
	return false
}
