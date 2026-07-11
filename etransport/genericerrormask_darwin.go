//go:build darwin

package etransport

import (
	"net"
	"syscall"
)

// errorMask filters macOS-specific UDP multicast errors that are
// non-fatal on Darwin platforms (e.g., ENETDOWN when the interface
// is temporarily unavailable). All other errors pass through unchanged.
//
// errorMask 过滤 macOS 特有的 UDP 多播错误，这些错误在 Darwin 平台上
// 是非致命的（例如接口暂时不可用时的 ENETDOWN）。其他错误原样返回。
func errorMask(err error) error {
	if oe, ok := err.(*net.OpError); ok {
		if se, ok := oe.Err.(*syscall.Errno); ok {
			switch *se {
			case syscall.ENETDOWN:
				// Interface is temporarily down; treat as non-fatal.
				return nil
			}
		}
	}
	return err
}