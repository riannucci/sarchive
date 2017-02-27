// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"io"

	"github.com/luci/luci-go/common/errors"
)

// Magic is the magic bytes which appear at the beginning of a sarchive.
const Magic = "SAR"

// Version is the version of the sarchive format.
const Version byte = 1

var magicVer []byte

func init() {
	magicVer = []byte(Magic + string(Version))
}

// WriteMagic writes SAR+VERSION to the writer.
func WriteMagic(w io.Writer) error {
	_, err := w.Write(magicVer)
	return err
}

// ReadMagic reads magic from the reader and checks that it's equal to
// SAR, and ensures that the file version is <= Version.
func ReadMagic(r io.Reader) (version byte, err error) {
	buf := make([]byte, 4)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}

	sBuf := string(buf[:3])
	if Magic != sBuf {
		err = errors.Reason("bad magic: %(magic)q").D("magic", sBuf).Err()
		return
	}

	version = buf[3]
	if version > Version {
		err = errors.Reason("bad version: %(ver)d > %(ours)d").
			D("ver", version).D("ours", Version).Err()
		return
	}

	return
}
