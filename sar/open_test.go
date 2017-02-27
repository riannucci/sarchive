// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sar

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/net/context"

	. "github.com/smartystreets/goconvey/convey"

	. "github.com/luci/luci-go/common/testing/assertions"

	"github.com/riannucci/sarchive/sar/sardata"
	"github.com/riannucci/sarchive/sar/sardata/toc"
)

func f(name string, size uint64) *toc.Entry {
	return &toc.Entry{Name: name, Etype: &toc.Entry_File{File: &toc.File{
		Size: size,
	}}}
}

func t(name string, entries ...*toc.Entry) *toc.Entry {
	return &toc.Entry{Name: name, Etype: &toc.Entry_Tree{Tree: &toc.Tree{
		Entries: entries,
	}}}
}

type nullWriteCloser struct {
	io.Writer
}

func (nullWriteCloser) Close() error { return nil }

type nullReadSeekCloser struct {
	io.ReadSeeker
}

func (nullReadSeekCloser) Close() error { return nil }

func TestOpen(tst *testing.T) {
	tst.Parallel()

	mockTOC := &toc.TOC{
		CaseSafe: true,
		Root: &toc.Tree{Entries: []*toc.Entry{
			f("someFile", 13),
			f("someOtherFile", 18),
			t("tree",
				f("subFile", 17),
			),
			f("lastFile", 13),
		}},
	}

	mockArchive := &bytes.Buffer{}
	csumWriter := sardata.ChecksumBLAKE2b.Writer(nullWriteCloser{mockArchive})

	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	must(sardata.WriteMagic(csumWriter))
	must(sardata.WriteTOC(csumWriter, mockTOC, sardata.CompressionFlate, 9))
	expectedTOC := make([]byte, mockArchive.Len()-4) // minus magic
	copy(expectedTOC, mockArchive.Bytes()[4:])
	bw, err := sardata.BlockWriter(csumWriter, sardata.CompressionFlate, 9)
	must(err)
	_, err = bw.Write([]byte("someFile data"))
	must(err)
	_, err = bw.Write([]byte("someOtherFile data"))
	must(err)
	_, err = bw.Write([]byte("tree/subFile data"))
	must(err)
	_, err = bw.Write([]byte("lastFile data"))
	must(err)
	must(bw.Close())
	must(csumWriter.Close())

	Convey("Open", tst, func() {
		Convey("standard", func() {
			ar, err := Open(nullReadSeekCloser{bytes.NewReader(mockArchive.Bytes())})
			So(err, ShouldBeNil)
			So(ar.TOC, ShouldResemble, mockTOC)
			_, err = ar.RawTOC()
			So(err, ShouldErrLike, "must supply WithRawTOC to Open to use RawTOC")
			So(ar.Close(), ShouldBeNil)
		})

		Convey("VerifyEarly", func() {
			ar, err := Open(nullReadSeekCloser{bytes.NewReader(mockArchive.Bytes())}, WithVerification(VerifyEarly))
			So(err, ShouldBeNil)
			So(ar.TOC, ShouldResemble, mockTOC)
			So(ar.Close(), ShouldBeNil)
		})

		Convey("VerifyNever", func() {
			newBytes := make([]byte, mockArchive.Len())
			copy(newBytes, mockArchive.Bytes())
			newBytes[len(newBytes)-10] = 0  // break the checksum
			newBytes[len(newBytes)-1] = 100 // break the 'seekback' value

			ar, err := Open(nullReadSeekCloser{bytes.NewReader(newBytes)}, WithVerification(VerifyNever))
			So(err, ShouldBeNil)
			So(ar.TOC, ShouldResemble, mockTOC)
			So(ar.Close(), ShouldBeNil)
		})

		Convey("CacheRawTOC", func() {
			ar, err := Open(nullReadSeekCloser{bytes.NewReader(mockArchive.Bytes())}, WithRawTOC(true))
			So(err, ShouldBeNil)
			So(ar.TOC, ShouldResemble, mockTOC)
			data, err := ar.RawTOC()
			So(err, ShouldBeNil)
			So(data, ShouldResemble, expectedTOC)
			So(ar.Close(), ShouldBeNil)
		})

		Convey("and unpack", func() {
			ar, err := Open(nullReadSeekCloser{bytes.NewReader(mockArchive.Bytes())})
			So(err, ShouldBeNil)

			dirName, err := ioutil.TempDir("", "")
			So(err, ShouldBeNil)
			defer os.RemoveAll(dirName)

			So(ar.UnpackTo(context.Background(), dirName), ShouldBeNil)

			hasContent := func(path interface{}, expect ...interface{}) string {
				data, err := ioutil.ReadFile(filepath.Join(dirName, path.(string)))
				if err != nil {
					return err.Error()
				}
				return ShouldResemble(string(data), expect[0].(string))
			}

			So("someFile", hasContent, "someFile data")
			So("someOtherFile", hasContent, "someOtherFile data")
			So("tree/subFile", hasContent, "tree/subFile data")
			So("lastFile", hasContent, "lastFile data")
		})
	})
}
