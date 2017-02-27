// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sar

import (
	"bytes"
	"hash"
	"io"
	"io/ioutil"

	"github.com/luci/luci-go/common/errors"

	"github.com/riannucci/sarchive/sar/sardata"
	"github.com/riannucci/sarchive/sar/sardata/toc"
)

type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// OpenedArchive represends an Open'd sar file.
type OpenedArchive struct {
	r io.ReadCloser

	didClose bool

	rawTOCBuf *bytes.Buffer
	TOC       *toc.TOC

	opts openOptionData
}

// RawTOC returns the raw bytes for the compressed TOC block if CacheRawTOC was
// provided.
func (a *OpenedArchive) RawTOC() ([]byte, error) {
	if a.rawTOCBuf != nil {
		return a.rawTOCBuf.Bytes(), nil
	}
	return nil, errors.New("must supply CacheRawTOC to Open to use RawTOC")
}

// Close closes the archive and the underlying reader. If UnpackTo hasn't been
// called, then this will also verify the checksum.
func (a *OpenedArchive) Close() error {
	if a.didClose {
		return nil
	}
	a.didClose = true

	if a.opts.earlyVerify {
		// already verified the checksum, so just close a.r
		return a.r.Close()
	}

	// otherwise we need to read to the end to check the checksum.
	// TODO(iannucci): this could overflow.
	var totalSize uint64
	a.TOC.LoopItems(func(path []string, ent *toc.Entry) error {
		if f := ent.GetFile(); f != nil {
			totalSize += f.Size
		}
		return nil
	})
	_, err := io.Copy(ioutil.Discard, io.LimitReader(a.r, int64(totalSize)))
	if err != nil {
		return err
	}

	return a.r.Close()
}

type openOptionData struct {
	earlyVerify  bool
	cacheRawTOC  bool
	unpackBuffer int
}

func (o openOptionData) setUpReader(r readSeekCloser) (ret io.ReadCloser, err error) {
	if o.earlyVerify {
		ret = io.ReadCloser(r)

		var h hash.Hash
		var nominalEnd int64
		var nominalCsum []byte
		_, h, nominalEnd, nominalCsum, err = sardata.ParseTrailer(r)
		if err != nil {
			err = errors.Annotate(err).Reason("early checksum setup").Err()
			return
		}
		var curLoctation int64
		if curLoctation, err = r.Seek(0, io.SeekCurrent); err != nil {
			err = errors.Annotate(err).Reason("early checksum seek").Err()
			return
		}
		if _, err = io.Copy(h, io.LimitReader(r, nominalEnd-curLoctation)); err != nil {
			err = errors.Annotate(err).Reason("early checksum calculation").Err()
			return
		}
		if !bytes.Equal(nominalCsum, h.Sum(nil)) {
			err = errors.Annotate(err).Reason("early checksum").Err()
			return
		}
		if _, err = r.Seek(curLoctation, io.SeekStart); err != nil {
			err = errors.Annotate(err).Reason("early checksum reset").Err()
			return
		}
	} else {
		ret, _, err = sardata.ChecksumReader(r)
	}
	return
}

// OpenOption functions can be supplied to the Open function
type OpenOption func(*openOptionData)

// EarlyVerify is an OpenOption which allows you to evaluate the checksum at the
// time the archive is opened, as opposed to lazily.
func EarlyVerify(o *openOptionData) {
	o.earlyVerify = true
}

// CacheRawTOC is an OpenOption which instructs Open to duplicate the raw manifest
// block. This can be useful for storing the manifest on disk next to the
// unpacked Archive, for example.
func CacheRawTOC(o *openOptionData) {
	o.cacheRawTOC = true
}

// UnpackBuffer is an OpenOption factory which indicates the number of bytes
// that UnpackTo will attempt to decompress ahead of time. Default if
// unspecified is 16MB.
func UnpackBuffer(factor int) OpenOption {
	return func(o *openOptionData) {
		o.unpackBuffer = factor
	}
}

// Open opens a SARchive from the given reader.
//
// It will read and validate the table of contents, and open the archive data
// block but not read any of the data.
//
// To get a positive confirmation for the integrity of the archive, you must
// call Close() and observe the error (or you can use EarlyVerify to get
// a preemptive integrity check).
//
// Options:
//   EarlyVerify - do all checksum validation up-front before any parsing
//     occurs. Use this mode if you don't have some external checksum for data
//     integrity.
//   CacheRawTOC - grab a copy of the raw compressed TOC. Can be used to
//     extract raw TOC to disk. TOC can be parsed with sardata.ReadTOC.
//   UnpackBuffer - set the size of the read-ahead decompression buffer in
//     bytes. Setting this to 0 will do an unbuffered (no readahead) unpack.
//     Defaults to 16MB if unspecified.
func Open(r readSeekCloser, options ...OpenOption) (ret *OpenedArchive, err error) {
	opts := openOptionData{
		unpackBuffer: 16 * 1024 * 1024, // 16MB
	}
	for _, o := range options {
		o(&opts)
	}

	openedReader, err := opts.setUpReader(r)
	if err != nil {
		return
	}

	var version byte
	if version, err = sardata.ReadMagic(openedReader); err != nil {
		err = errors.Annotate(err).Reason("checking magic").Err()
		return
	}
	if version != 1 {
		err = errors.Reason("unsupported version %(version)d").
			D("version", version).Err()
		return
	}

	ar := &OpenedArchive{
		r:    openedReader,
		opts: opts,
	}

	tocReader := io.Reader(openedReader)
	if opts.cacheRawTOC {
		ar.rawTOCBuf = &bytes.Buffer{}
		tocReader = io.TeeReader(openedReader, ar.rawTOCBuf)
	}

	if ar.TOC, err = sardata.ReadTOC(tocReader); err != nil {
		err = errors.Annotate(err).Reason("reading TOC").Err()
		return
	}

	ar.r, err = sardata.BlockReader(openedReader)
	if err != nil {
		err = errors.Annotate(err).Reason("opening data block").Err()
		return
	}

	ret = ar
	return
}
