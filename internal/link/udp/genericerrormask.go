//go:build !darwin

package udp

// errorMask is a no-op on non-Darwin platforms. It simply returns the
// original error unchanged.
func errorMask(err error) error {
	return err
}
