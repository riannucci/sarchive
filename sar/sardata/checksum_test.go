// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

type readSeekCloseHook struct {
	io.ReadSeeker

	clsFn func() error
}

func (c readSeekCloseHook) Close() error {
	return c.clsFn()
}

func TestChecksum(t *testing.T) {
	t.Parallel()

	Convey("Checksum", t, func() {
		Convey("write", func() {
			buf := &bytes.Buffer{}
			closed := false
			wr := ChecksumSHA2_256.Writer(writeCloseHook{
				buf,
				func() error {
					closed = true
					return nil
				},
			})
			_, err := wr.Write([]byte("hello world!"))
			So(err, ShouldBeNil)
			So(wr.Close(), ShouldBeNil)

			Convey("ok", func() {
				So(closed, ShouldBeTrue)
				payload := []byte("hello world!")
				payload = append(payload, 1) // ChecksumSHA2_256
				sum := sha256.Sum256([]byte("hello world!"))
				payload = append(payload, sum[:]...)
				payload = append(payload, 32)
				So(buf.Bytes(), ShouldResemble, payload)
			})

			Convey("ParseTrailer", func() {
				Convey("normal", func() {
					closed := false
					c, h, nominalEnd, nominalCsum, err := ParseTrailer(readSeekCloseHook{
						bytes.NewReader(buf.Bytes()),
						func() error {
							closed = true
							return nil
						},
					})
					So(err, ShouldBeNil)
					So(c, ShouldEqual, ChecksumSHA2_256)
					So(h, ShouldResemble, sha256.New())
					So(nominalEnd, ShouldEqual, len("hello world!"))
					sum := sha256.Sum256([]byte("hello world!"))
					So(nominalCsum, ShouldResemble, sum[:])
				})

				Convey("bad size", func() {
					// Change to SHA2-512
					buf.Bytes()[len("hello world!")] = 2
					_, _, _, _, err := ParseTrailer(readSeekCloseHook{
						bytes.NewReader(buf.Bytes()),
						nil,
					})
					So(err, ShouldErrLike, "mismatched hash size (ChecksumSHA2_512): 32 expected 64")
				})

				Convey("bad scheme", func() {
					buf.Bytes()[len("hello world!")] = 100
					_, _, _, _, err := ParseTrailer(readSeekCloseHook{
						bytes.NewReader(buf.Bytes()),
						nil,
					})
					So(err, ShouldErrLike, "Unknown checksum scheme 0x64")
				})
			})

			Convey("reader", func() {
				closed := false
				rc, c, err := ChecksumReader(readSeekCloseHook{
					bytes.NewReader(buf.Bytes()),
					func() error {
						closed = true
						return nil
					},
				})
				So(err, ShouldBeNil)
				So(c, ShouldEqual, ChecksumSHA2_256)
				So(rc, ShouldNotBeNil)

				Convey("normal", func() {
					newBuf := bytes.Buffer{}
					_, err = io.Copy(&newBuf, rc)
					So(err, ShouldBeNil)
					So(newBuf.String(), ShouldResemble, "hello world!")
					So(rc.Close(), ShouldBeNil)
					So(closed, ShouldBeTrue)
				})

				Convey("short read", func() {
					newBuf := make([]byte, 5)
					_, err = io.ReadFull(rc, newBuf)
					So(err, ShouldBeNil)
					So(newBuf, ShouldResemble, []byte("hello"))
					So(rc.Close(), ShouldErrLike, "junk after payload (7 bytes)")
				})

				Convey("bad checksum", func() {
					// modify underlying byte stream
					buf.Bytes()[0] = 'd'

					newBuf := bytes.Buffer{}
					_, err = io.Copy(&newBuf, rc)
					So(err, ShouldBeNil)
					So(newBuf.String(), ShouldResemble, "dello world!")
					So(rc.Close(), ShouldErrLike, "mismatched checksum (ChecksumSHA2_256)")
				})

			})
		})

		Convey("null", func() {
			buf := &bytes.Buffer{}
			closed := false
			wr := ChecksumNULL.Writer(writeCloseHook{
				buf,
				func() error {
					closed = true
					return nil
				},
			})
			_, err := wr.Write([]byte("hello world!"))
			So(err, ShouldBeNil)
			So(wr.Close(), ShouldBeNil)

			Convey("ok", func() {
				So(closed, ShouldBeTrue)
				payload := []byte("hello world!\xff\x00")
				So(buf.Bytes(), ShouldResemble, payload)

				Convey("reader", func() {
					closed := false
					rc, c, err := ChecksumReader(readSeekCloseHook{
						bytes.NewReader(buf.Bytes()),
						func() error {
							closed = true
							return nil
						},
					})
					So(err, ShouldBeNil)
					So(c, ShouldEqual, ChecksumNULL)
					So(rc, ShouldNotBeNil)

					Convey("normal", func() {
						newBuf := bytes.Buffer{}
						_, err = io.Copy(&newBuf, rc)
						So(err, ShouldBeNil)
						So(newBuf.String(), ShouldResemble, "hello world!")
						So(rc.Close(), ShouldBeNil)
						So(closed, ShouldBeTrue)
					})

					Convey("short read", func() {
						newBuf := make([]byte, 5)
						_, err = io.ReadFull(rc, newBuf)
						So(err, ShouldBeNil)
						So(newBuf, ShouldResemble, []byte("hello"))
						So(rc.Close(), ShouldErrLike, "junk after payload (7 bytes)")
					})
				})
			})
		})
	})
}
