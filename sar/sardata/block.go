// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/iotools"
)

// BlockHeader is used as the prefix to a block.
type BlockHeader struct {
	// Length is the number of compressed bytes compose the block, after the end
	// of this header.
	Length uint64

	// Compression indicates the compression decoder scheme that should be used
	// for the block.
	Compression CompressionScheme
}

func (b BlockHeader) Write(w io.Writer) error {
	buf := make([]byte, binary.MaxVarintLen64+1)
	buf = buf[:binary.PutUvarint(buf, b.Length)]
	buf = append(buf, byte(b.Compression))
	_, err := w.Write(buf)
	return err
}

func (b *BlockHeader) Read(r io.Reader) (err error) {
	br := byteReader{Reader: r}

	if b.Length, err = binary.ReadUvarint(br); err != nil {
		return
	}
	c, err := br.ReadByte()
	if err != nil {
		return
	}
	b.Compression = CompressionScheme(c)
	return b.Compression.Valid()
}

// BlockWriter returns a writer that will compress the data given to it. When
// the returned writer is closed, a BlockHeader and the compressed data will be
// written to `w`.
//
// This means that the compressed data is unfortunately buffered in memory so
// that the length of the compressed data can be calculated to put into the
// header.
func BlockWriter(w io.Writer, scheme CompressionScheme, level int) (io.WriteCloser, error) {
	buf := bytes.Buffer{}
	compressWriter, err := scheme.Writer(&buf, level)
	if err != nil {
		return nil, err
	}
	countWriter := &iotools.CountingWriter{Writer: compressWriter}

	return writeCloseHook{
		countWriter,
		func() error {
			if err := compressWriter.Close(); err != nil {
				return err
			}
			h := BlockHeader{uint64(buf.Len()), scheme}
			if err := h.Write(w); err != nil {
				return err
			}
			_, err := w.Write(buf.Bytes())
			return err
		},
	}, nil
}

// BlockReader expects to read a compressed block from r. If it finds one and
// knows the compression scheme of the block, it will return a ReadCloser which
// can be used to read the decompressed data from the block.
func BlockReader(r io.Reader) (io.ReadCloser, error) {
	h := BlockHeader{}
	if err := h.Read(r); err != nil {
		return nil, err
	}
	if h.Length > math.MaxInt64 {
		return nil, errors.New("block length exceeds int64")
	}
	return h.Compression.Reader(io.LimitReader(r, int64(h.Length)))
}
