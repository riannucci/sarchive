// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sardata

import (
	"io"
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	"github.com/riannucci/sarchive/sar/sardata/toc"
)

// WriteTOC writes a compressed table of contents to the given writer.
func WriteTOC(w io.Writer, t *toc.TOC, scheme CompressionScheme, level int) (err error) {
	var buf []byte
	if buf, err = proto.Marshal(t); err != nil {
		return
	}
	wc, err := BlockWriter(w, scheme, level)
	if err != nil {
		return
	}
	if _, err = wc.Write(buf); err != nil {
		return
	}
	return wc.Close()
}

// ReadTOC parsses a compressed table of contents from the given reader.
func ReadTOC(r io.Reader) (ret *toc.TOC, err error) {
	br, err := BlockReader(r)
	if err != nil {
		return nil, err
	}
	buf, err := ioutil.ReadAll(br)
	if err != nil {
		return
	}
	ret = &toc.TOC{}
	if err = proto.Unmarshal(buf, ret); err == nil {
		err = ret.Validate()
	}
	return
}
