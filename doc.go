// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package sarchive implements a simple 'solid' archive format which contains
// a separately extractable table of contents, somewhat comparable to ZIP or
// XAR. Unlike ZIP, the table of contents is compressed, and the file data is
// compressed in a single solid block (allowing better compression ratios for
// archives of similar files). Unlike XAR, the table of contents uses
// a relatively efficient and easy-to-manipulate protobuf instead of XML.
//
// Unlike ZIP and XAR both, the SAR format does not attempt to preserve or
// restore specific file ACLs (i.e. user ownership ids, posix mode, etc.), but
// instead allows files to set platform-specific mode bits (like the 'system'
// bit on windows), and express a small handful of cross-platform concepts (like
// 'read-only'). The reasoning is that, in this burgeoning age of the internet,
// porting user ids or mode flags across systems is silly, at best, and possibly
// harmful.
//
// It has a fairly basic format:
//   * file magic header ("SAR" + byte(API_VERSION)). API_VERSION current == 1.
//   * block_header + table_of_contents
//   * block_header + archive_data
//   * checksum
//
// block_headers define the compression type and length of subsequent block.
//
// table_of_contents is a protobuf defined in toc/toc.proto.
//
// archive_data is all of the file data noted in the table_of_contents,
// concatenated and compressed. The offsets and sizes in table_of_contents refer
// to locations in the uncompressed archive_data stream.
//
// checksum indicates the type of checksum (SHA2-512 is the only supported
// checksum at the time of writing), followed by the bytes of the checksum,
// followed by the length of the checksum (as a single byte). The checksum
// covers all bytes in the archive which preceed the checksum type inidicator.
// The length at the end of the checksum allows the checksum to be validated
// simply by reading from the end of the archive without parsing it.
//
// TODO(riannucci): implement better compression scheme like brotli or zstd...
// this depends on better compression support becoming available in golang.
//
// TODO(riannucci): if the format requires greater extensibility, implement
// a PNG-like chunk scheme (i.e. with FourCC identifiers for sections).
package sarchive
