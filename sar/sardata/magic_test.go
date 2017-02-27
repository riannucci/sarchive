// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"bytes"
	"io"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMagic(t *testing.T) {
	t.Parallel()

	Convey("Magic", t, func() {
		Convey("write", func() {
			buf := &bytes.Buffer{}
			So(WriteMagic(buf), ShouldBeNil)
			So(buf.Bytes(), ShouldResemble, []byte{'S', 'A', 'R', 1})
		})

		Convey("read", func() {
			Convey("good", func() {
				Convey("matching version", func() {
					buf := bytes.NewReader([]byte{'S', 'A', 'R', 1})
					v, err := ReadMagic(buf)
					So(err, ShouldBeNil)
					So(v, ShouldEqual, 1)
				})

				Convey("older version", func() {
					buf := bytes.NewReader([]byte{'S', 'A', 'R', 0})
					v, err := ReadMagic(buf)
					So(err, ShouldBeNil)
					So(v, ShouldEqual, 0)
				})
			})

			Convey("bad", func() {
				Convey("bad prefix", func() {
					buf := bytes.NewReader([]byte{'P', 'K', 3, 4})
					_, err := ReadMagic(buf)
					So(err, ShouldErrLike, `bad magic: "PK\x03"`)
				})

				Convey("newer version", func() {
					buf := bytes.NewReader([]byte{'S', 'A', 'R', 4})
					_, err := ReadMagic(buf)
					So(err, ShouldErrLike, `bad version: 4 > 1`)
				})

				Convey("short read", func() {
					buf := bytes.NewReader([]byte{'S', 'A'})
					_, err := ReadMagic(buf)
					So(err, ShouldErrLike, io.ErrUnexpectedEOF)
				})

			})
		})
	})
}
