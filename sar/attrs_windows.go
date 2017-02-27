// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build windows

package sar

import (
	"syscall"

	"github.com/riannucci/sarchive/sar/sardata/toc"
)

// See GetFileAttributes for values.
const (
	winAttrHidden = 0x2
	winAttrSystem = 0x4
)

func setWinFileAttributes(path string, m *toc.WinMode) error {
	if m == nil {
		return nil
	}
	var attrs uint32
	if m.Hidden {
		attrs |= winAttrHidden
	}
	if m.System {
		attrs |= winAttrSystem
	}
	if attrs == 0 {
		return nil
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(p, uint32(attrs))
}
