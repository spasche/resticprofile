//go:build darwin

package platform

func IsDarwin() bool {
	return true
}

func IsWindows() bool {
	return false
}

func SupportsSyslog() bool {
	return true
}
