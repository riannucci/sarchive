// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sar

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"

	"github.com/riannucci/sarchive/sar/sardata/toc"
)

func ensureRoot(root string) error {
	if st, err := os.Stat(root); !os.IsNotExist(err) {
		return err
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0777); err != nil {
			return errors.Annotate(err).Reason("making root dir").Err()
		}
	} else if !st.IsDir() {
		return err
	} else if st.IsDir() {
		f, err := os.Open(root)
		if err != nil {
			return err
		}
		finfos, err := f.Readdir(1)
		f.Close()
		if err != nil {
			return err
		}
		if len(finfos) != 0 {
			return errors.New("dir not empty")
		}
	}
	return nil
}

func ensureSymlink(wg *sync.WaitGroup, ech chan<- error, abs, rel string, s *toc.SymLink) {
	target := filepath.Join(s.Target...)
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := errors.Annotate(os.Symlink(target, abs)).
			Reason("writing symlink %(rel)q -> %(target)q").
			D("rel", rel).D("target", target).Err()
		ech <- err
	}()
}

func ensureFile(syncBuf []byte, wg *sync.WaitGroup, ech chan<- error, abs, rel string, r io.Reader, file *toc.File) {
	f, err := os.Create(abs)
	if err != nil {
		ech <- errors.Annotate(err).Reason("creating file %(rel)q").
			D("rel", rel).Err()
		return
	}
	st, err := f.Stat()
	if err != nil {
		ech <- errors.Annotate(err).Reason("statting file %(rel)q").
			D("rel", rel).Err()
		return
	}
	// must copy in main goroutine because all files are sequential in
	// r (and there's no seek method). However, we don't need to
	// block on stat'ing/closing the file.
	if _, err := io.CopyBuffer(f, io.LimitReader(r, int64(file.Size)), syncBuf); err != nil {
		ech <- errors.Annotate(err).Reason("writing file %(rel)q").
			D("rel", rel).Err()
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		mode := st.Mode()
		if file.GetPosixMode().GetExecutable() {
			mode |= 0111 // ugo+x
		}
		if file.GetCommonMode().GetReadonly() {
			mode &= 0555 // ugo-r
		}
		if err := f.Chmod(mode); err != nil {
			ech <- errors.Annotate(err).Reason("setting mode %(rel)q").
				D("rel", rel).Err()
		}
		if err := setWinFileAttributes(abs, file.GetWinMode()); err != nil {
			ech <- errors.Annotate(err).Reason("setting windows mode %(rel)q").
				D("rel", rel).Err()
		}
		ech <- errors.Annotate(f.Close()).Reason("closing file %(rel)q").
			D("rel", rel).Err()
	}()
}

func (a *OpenedArchive) prepReader() (io.Reader, io.Closer, error) {
	dataReader := io.Reader(a.r)
	checksumCloser := io.Closer(a.r)
	if a.opts.unpackBufferSize > 0 {
		rd, wr := io.Pipe()
		go func(r io.Reader) {
			_, err := bufio.NewReaderSize(r, a.opts.unpackBufferSize).WriteTo(wr)
			wr.CloseWithError(err)
		}(dataReader)
		dataReader = rd
	}

	return dataReader, checksumCloser, nil
}

// UnpackTo does a streaming unpack of the entire Archive to the provided
// location.
//
// root must be either a non-existant path, or a path to an empty directory.
//
// It is invalid to call UnpackTo twice, or to call it on a Close()'d Archive.
func (a *OpenedArchive) UnpackTo(ctx context.Context, root string) error {
	if a.didClose {
		return errors.New("can only unpack once/cannot unpack closed Archive")
	}
	a.didClose = true

	root, err := filepath.Abs(root)
	if err != nil {
		return errors.Annotate(err).Reason("making abspath").Err()
	}

	if err := ensureRoot(root); err != nil {
		return errors.Annotate(err).Reason("checking root").Err()
	}

	dataReader, checksumCloser, err := a.prepReader()
	if err != nil {
		return errors.Annotate(err).Reason("prepping reader").Err()
	}

	ech := make(chan error, 1)
	go func() {
		defer close(ech)

		wg := &sync.WaitGroup{}
		defer wg.Wait()

		syncBuf := make([]byte, 32*1024)

		ech <- a.TOC.LoopItems(func(path []string, ent *toc.Entry) error {
			rel := filepath.Join(path...)
			abs := filepath.Join(root, rel)

			switch x := ent.Etype.(type) {
			case *toc.Entry_Tree:
				if err := os.Mkdir(abs, 0777); err != nil {
					// this immediately quits the loop
					return errors.Annotate(err).Reason("FATAL: making dir %(rel)q").
						D("rel", rel).Err()
				}

			case *toc.Entry_Symlink:
				ensureSymlink(wg, ech, abs, rel, x.Symlink)

			case *toc.Entry_File:
				ensureFile(syncBuf, wg, ech, abs, rel, dataReader, x.File)

			default:
				panic("impossible!")
			}
			return nil
		})
	}()

	hadError := false
	for err := range ech {
		if err == nil {
			continue
		}
		if !hadError {
			logging.Errorf(ctx, "errors while unpacking to %q:", root)
			hadError = true
		}
		logging.Errorf(ctx, "  %s", err)
	}
	if hadError {
		return errors.New("errors while unpacking (see log)")
	}

	return checksumCloser.Close()
}
