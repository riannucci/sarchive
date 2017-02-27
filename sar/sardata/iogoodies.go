// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import "io"

type writeCloseHook struct {
	io.Writer

	clsFn func() error
}

func (c writeCloseHook) Close() error {
	if c.clsFn != nil {
		return c.clsFn()
	}
	return nil
}

type readCloseHook struct {
	io.Reader

	clsFn func() error
}

func (c readCloseHook) Close() error {
	if c.clsFn != nil {
		return c.clsFn()
	}
	return nil
}

type byteReader struct {
	io.Reader
	buf [1]byte
}

func (b byteReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(b.Reader, b.buf[:])
	return b.buf[0], err
}
