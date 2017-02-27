// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

//go:generate stringer -type ChecksumScheme

package sardata

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/sha3"

	"github.com/luci/luci-go/common/errors"
)

// ChecksumScheme are the various checksum types known to the sarchive format.
type ChecksumScheme byte

// These are the available checksum algorithms implemented for sarchives.
const (
	ChecksumSHA2_256 ChecksumScheme = iota + 1
	ChecksumSHA2_512
	ChecksumBLAKE2s
	ChecksumBLAKE2b
	ChecksumSHA3_256
	ChecksumSHA3_512

	// Bypasses ALL checksum verification.
	ChecksumNULL ChecksumScheme = 255
)

// Valid returns nil iff the ChecksumScheme is valid.
func (c ChecksumScheme) Valid() error {
	switch c {
	case ChecksumSHA2_256:
	case ChecksumSHA2_512:
	case ChecksumBLAKE2s:
	case ChecksumBLAKE2b:
	case ChecksumSHA3_256:
	case ChecksumSHA3_512:
	case ChecksumNULL:
	default:
		return errors.Reason("Unknown checksum scheme 0x%(c)x").D("c", byte(c)).Err()
	}
	return nil
}

// nullHash is so that ChecksumScheme.Hash returns a valid hash.Hash. However,
// Writer and Reader both have special optimizations for the null hash.
type nullHash struct{}

var _ hash.Hash = nullHash{}

func (nullHash) Reset()                    {}
func (nullHash) BlockSize() int            { return 0 }
func (nullHash) Size() int                 { return 0 }
func (nullHash) Sum(buf []byte) []byte     { return buf }
func (nullHash) Write([]byte) (int, error) { return 0, nil }

// Hash gets the Hash interface associated with this scheme.
func (c ChecksumScheme) Hash() hash.Hash {
	var h hash.Hash
	switch c {
	case ChecksumSHA2_256:
		h = sha256.New()
	case ChecksumSHA2_512:
		h = sha512.New()
	case ChecksumBLAKE2s:
		h, _ = blake2s.New256(nil)
	case ChecksumBLAKE2b:
		h, _ = blake2b.New512(nil)
	case ChecksumSHA3_256:
		h = sha3.New256()
	case ChecksumSHA3_512:
		h = sha3.New512()
	case ChecksumNULL:
		h = nullHash{}
	}
	if h == nil {
		panic(c.Valid())
	}
	if h.Size() > 255 {
		panic("selected checksum has a size over 255?")
	}
	return h
}

// Writer converts the provided WriteCloser into a new WriteCloser which will
// write a checksum footer when it is Close()'d.
func (c ChecksumScheme) Writer(w io.WriteCloser) io.WriteCloser {
	if c == ChecksumNULL {
		return writeCloseHook{
			w,
			func() (err error) {
				if _, err = w.Write([]byte{byte(c), 0}); err == nil {
					return w.Close()
				}
				return err
			},
		}
	}

	h := c.Hash()

	return writeCloseHook{
		io.MultiWriter(w, h),
		func() (err error) {
			buf := make([]byte, 0, h.Size()+2)
			buf = append(buf, byte(c))
			buf = h.Sum(buf)
			buf = append(buf, byte(h.Size()))
			if _, err = w.Write(buf); err == nil {
				return w.Close()
			}
			return err
		},
	}
}

type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// ErrMismatchedChecksum is returned from ChecksumReader if the checksum doesn't
// match up.
type ErrMismatchedChecksum struct {
	Scheme  ChecksumScheme
	Nominal []byte
	Actual  []byte
}

func (e *ErrMismatchedChecksum) Error() string {
	return fmt.Sprintf("mismatched checksum (%s): %x expected %x", e.Scheme,
		e.Nominal, e.Actual)
}

// ParseTrailer seeks to the end of r, parses the checksum trailer, and returns
// the pertinent details.
//
// Note that nominalEnd is an offset from the beginning of the FILE (not the
// current position of r!), as defined by io.Seeker.
func ParseTrailer(r readSeekCloser) (c ChecksumScheme, h hash.Hash, nominalEnd int64, nominalChecksum []byte, err error) {
	curOffset, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	if _, err = r.Seek(-1, io.SeekEnd); err != nil {
		return
	}
	one := []byte{0}
	if _, err = io.ReadFull(r, one); err != nil {
		return
	}

	nominalSize := one[0]
	// +1 for nominalSize (we just read)
	// +nominalEnd for checksum
	// +1 for ChecksumScheme
	if nominalEnd, err = r.Seek(-(int64(nominalSize) + 2), io.SeekCurrent); err != nil {
		return
	}
	buf := make([]byte, nominalSize+1)
	if _, err = io.ReadFull(r, buf); err != nil {
		return
	}

	c = ChecksumScheme(buf[0])
	nominalChecksum = buf[1:]
	if err = c.Valid(); err != nil {
		return
	}
	h = c.Hash()
	if int(nominalSize) != h.Size() {
		err = errors.
			Reason("mismatched hash size (%(csum)s): %(nominal)d expected %(actual)d").
			D("csum", c).D("nominal", nominalSize).D("actual", h.Size()).Err()
		return
	}

	// finally seak back to where we started
	_, err = r.Seek(curOffset, io.SeekStart)
	return
}

// ChecksumReader returns a ReadCloser which will verify the trailing checksum
// of the stream contained by readSeekCloser. It assumes the beginning of the
// checksum range is the current position of the readSeekCloser.
//
// The checksum verification will happen when the returned Reader is Close()'d.
func ChecksumReader(r readSeekCloser) (ret io.ReadCloser, c ChecksumScheme, err error) {
	c, h, nominalEnd, nominalChecksum, err := ParseTrailer(r)
	if c == ChecksumNULL {
		return readCloseHook{
			io.LimitReader(r, nominalEnd),
			func() error {
				actualEnd, err := r.Seek(0, io.SeekCurrent)
				if err != nil {
					return err
				}
				if actualEnd != nominalEnd {
					return errors.
						Reason("junk after payload (%(diff)d bytes): 0x%(nominal)x to 0x(actual)x").
						D("nominal", nominalEnd).D("actual", actualEnd).
						D("diff", nominalEnd-actualEnd).Err()
				}
				return r.Close()
			},
		}, c, nil
	}

	ret = readCloseHook{
		io.TeeReader(io.LimitReader(r, nominalEnd), h),
		func() error {
			actualEnd, err := r.Seek(0, io.SeekCurrent)
			if err != nil {
				return err
			}
			if actualEnd != nominalEnd {
				return errors.
					Reason("junk after payload (%(diff)d bytes): 0x%(nominal)x to 0x(actual)x").
					D("nominal", nominalEnd).D("actual", actualEnd).
					D("diff", nominalEnd-actualEnd).Err()
			}
			actualChecksum := h.Sum(nil)
			if !bytes.Equal(actualChecksum, nominalChecksum) {
				return &ErrMismatchedChecksum{c, nominalChecksum, actualChecksum}
			}
			return r.Close()
		},
	}
	return
}
