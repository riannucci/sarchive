// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"compress/flate"
	"io"

	"github.com/luci/luci-go/common/errors"
)

// CompressionScheme indicates the type of compression used in a block, as
// indicated by that block's BlockHeader.
type CompressionScheme byte

// These are the currently supported compressions schemes.
//
// TODO(iannucci): add zstd or brotli as support becomes available.
const (
	CompressionNone CompressionScheme = iota + 1
	CompressionFlate
)

// Writer returns a new compressing writer for the given scheme.
func (c CompressionScheme) Writer(w io.Writer, level int) (io.WriteCloser, error) {
	switch c {
	case CompressionNone:
		return writeCloseHook{w, nil}, nil
	case CompressionFlate:
		return flate.NewWriter(w, level)
	}
	return nil, c.Valid()
}

// Reader returns a new decompressing reader for the given scheme.
func (c CompressionScheme) Reader(r io.Reader) (io.ReadCloser, error) {
	switch c {
	case CompressionNone:
		return readCloseHook{r, nil}, nil
	case CompressionFlate:
		return flate.NewReader(r), nil
	}
	return nil, c.Valid()
}

// Valid returns a nil err iff this CompressionScheme is valid.
func (c CompressionScheme) Valid() error {
	switch c {
	case CompressionNone, CompressionFlate:
		return nil
	}
	return errors.Reason("Unknown compression scheme %(c)x").D("c", c).Err()
}
