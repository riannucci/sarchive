// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sar

import (
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/riannucci/sarchive/sar/sardata"
	"github.com/riannucci/sarchive/sar/sardata/toc"
)

type createOptionData struct {
	compressKind  sardata.CompressionScheme
	compressLevel int
	checksumKind  sardata.ChecksumScheme
}

type CreateOption func(*createOptionData)

func WithCompression(kind sardata.CompressionScheme, level int) CreateOption {
	return func(o *createOptionData) {
		o.compressKind = kind
		o.compressLevel = level
	}
}

func WithChecksum(kind sardata.ChecksumScheme) CreateOption {
	return func(o *createOptionData) {
		o.checksumKind = kind
	}
}

func GenerateTreeFromPath(path string) (*toc.TOC, bool, error) {
	return nil, false, nil
}

func CreateFromPath(out io.Writer, path string, options ...CreateOption) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	defaultChecksum := sardata.ChecksumSHA2_256
	if runtime.GOARCH == "amd64" {
		defaultChecksum = sardata.ChecksumSHA2_512
	}

	opts := createOptionData{
		compressKind:  sardata.CompressionFlate,
		compressLevel: 9,
		checksumKind:  defaultChecksum,
	}
	for _, o := range options {
		o(&opts)
	}

	if err := sardata.WriteMagic(out); err != nil {
		return err
	}

	toc := &toc.TOC{}
	_ = toc
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_ = f

	return nil
}
