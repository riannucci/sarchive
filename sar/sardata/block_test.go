// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"bytes"
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBlock(t *testing.T) {
	t.Parallel()

	Convey("Block", t, func() {
		closed := false
		buf := &bytes.Buffer{}
		wc, err := BlockWriter(writeCloseHook{
			buf,
			func() error {
				closed = true
				return nil
			},
		}, CompressionFlate, 9)
		So(err, ShouldBeNil)
		_, err = wc.Write(bytes.Repeat([]byte("hello world!"), 100))
		So(err, ShouldBeNil)
		So(wc.Close(), ShouldBeNil)

		Convey("normal", func() {
			So(buf.Bytes(), ShouldResemble, []byte{
				28,                                           // compressed length (uvarint)
				2,                                            // compression type
				202, 72, 205, 201, 201, 87, 40, 207, 47, 202, // data
				73, 81, 28, 101, 143, 178, 71, 217, 163, 236,
				193, 204, 6, 4, 0, 0, 255, 255,
			})
		})

		Convey("reader", func() {
			rc, err := BlockReader(bytes.NewReader(buf.Bytes()))
			So(err, ShouldBeNil)
			newBuf := bytes.Buffer{}
			_, err = io.Copy(&newBuf, rc)
			So(err, ShouldBeNil)
			So(rc.Close(), ShouldBeNil)
			So(newBuf.Bytes(), ShouldResemble, bytes.Repeat([]byte("hello world!"), 100))
		})
	})
}
