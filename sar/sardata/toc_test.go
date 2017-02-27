// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"bytes"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/riannucci/sarchive/sar/sardata/toc"
)

func TestTOC(t *testing.T) {
	t.Parallel()

	Convey("TOC", t, func() {
		buf := &bytes.Buffer{}
		t := &toc.TOC{
			Root: &toc.Tree{Entries: []*toc.Entry{
				{Name: "foo", Etype: &toc.Entry_File{File: &toc.File{}}},
				{Name: "bar", Etype: &toc.Entry_Tree{Tree: &toc.Tree{Entries: []*toc.Entry{
					{Name: "sub", Etype: &toc.Entry_File{File: &toc.File{}}},
				}}}},
			}},
		}
		So(WriteTOC(buf, t, CompressionFlate, 9), ShouldBeNil)

		Convey("ok", func() {
			So(buf.Bytes(), ShouldResemble, []byte{
				33,                                      // payload size
				2,                                       // compression scheme
				18, 146, 230, 98, 231, 98, 78, 203, 207, // data
				23, 98, 224, 18, 224, 98, 78, 74, 44,
				82, 226, 4, 137, 20, 151, 38, 9, 49, 0,
				2, 0, 0, 255, 255,
			})
		})

		Convey("read", func() {
			newT, err := ReadTOC(buf)
			So(err, ShouldBeNil)
			So(newT, ShouldResemble, t)
		})
	})
}
