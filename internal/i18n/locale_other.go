//go:build !windows

package i18n

// osLocale has nothing beyond the POSIX variables already read.
func osLocale() string { return "" }
