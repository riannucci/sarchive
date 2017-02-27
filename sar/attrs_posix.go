// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// +build !windows

package sar

import (
	"github.com/riannucci/sarchive/sar/sardata/toc"
)

func setWinFileAttributes(path string, m *toc.WinMode) error {
	return nil
}
