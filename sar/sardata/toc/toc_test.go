// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package toc

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestTOCNormalize(t *testing.T) {
	t.Parallel()

	Convey("TOC.Validate", t, func() {
		Convey("SymLink.Validate", func() {
			Convey("good", func() {
				Convey("non-rel", func() {
					s := &SymLink{[]string{"some", "path", "file.ext"}}
					So(s.Validate(0), ShouldBeNil)
				})

				Convey("relative", func() {
					s := &SymLink{[]string{"some", "..", "file.ext"}}
					So(s.Validate(1), ShouldBeNil)
				})
			})

			Convey("bad", func() {
				Convey("empty", func() {
					s := &SymLink{}
					So(s.Validate(0), ShouldErrLike, "empty")
				})

				Convey("bad piece", func() {
					s := &SymLink{[]string{"path", "to", "some|invalid"}}
					So(s.Validate(0), ShouldErrLike, `bad char "|"`)

					s = &SymLink{[]string{"path", "", "some|invalid"}}
					So(s.Validate(0), ShouldErrLike, `empty path component`)

					s = &SymLink{[]string{".", "buh", "something"}}
					So(s.Validate(0), ShouldErrLike, `'.' path component`)
				})

				Convey("bad relative", func() {
					s := &SymLink{[]string{"..", "..", "file"}}
					So(s.Validate(0), ShouldErrLike, `escapes root`)
					So(s.Validate(1), ShouldErrLike, `escapes root`)
					So(s.Validate(2), ShouldErrLike)
				})
			})
		})

		Convey("Tree.Validate", func() {
			Convey("good", func() {
				Convey("caseSafe", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"someSymlink", &Entry_Symlink{&SymLink{[]string{"someFile"}}}},
						{"someTree", &Entry_Tree{&Tree{[]*Entry{
							{"subFile", &Entry_File{}},
							{"subSymlink", &Entry_Symlink{&SymLink{[]string{"..", "someSymlink"}}}},
						}}}},
					}}
					So(t.Validate(true, 0), ShouldBeNil)
				})

				Convey("not caseSafe", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"SOMEFILE", &Entry_File{}},
						{"someSymlink", &Entry_Symlink{&SymLink{[]string{"someFile"}}}},
						{"someTree", &Entry_Tree{&Tree{[]*Entry{
							{"subFile", &Entry_File{}},
							{"subSymlink", &Entry_Symlink{&SymLink{[]string{"..", "someSymlink"}}}},
						}}}},
					}}
					So(t.Validate(false, 0), ShouldBeNil)
				})
			})

			Convey("bad", func() {
				Convey("duplicate", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"someFile", &Entry_File{}},
					}}
					So(t.Validate(true, 0), ShouldErrLike, "duplicate entry")
				})

				Convey("not caseSafe", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"SOMEFILE", &Entry_File{}},
					}}
					So(t.Validate(true, 0), ShouldErrLike, "case-sensitive")
				})

				Convey("bad entry name", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"invalid:file", &Entry_File{}},
					}}
					So(t.Validate(true, 0), ShouldErrLike, `bad char ":"`)
				})

				Convey("relative entry name", func() {
					t := &Tree{[]*Entry{
						{"someFile", &Entry_File{}},
						{"..", &Entry_File{}},
					}}
					So(t.Validate(true, 0), ShouldErrLike, `relative path segment`)
				})
			})
		})
	})
}

func TestTOCLoop(t *testing.T) {
	t.Parallel()

	Convey("TOC.LoopItems", t, func() {
		t := &TOC{true, &Tree{[]*Entry{
			{"someFile", &Entry_File{}},
			{"someSymlink", &Entry_Symlink{&SymLink{[]string{"someFile"}}}},
			{"someTree", &Entry_Tree{&Tree{[]*Entry{
				{"subFile", &Entry_File{}},
				{"subSymlink", &Entry_Symlink{&SymLink{[]string{"..", "someSymlink"}}}},
			}}}},
			{"otherFile", &Entry_File{}},
			{"lastFile", &Entry_File{}},
		}}}
		So(t.Validate(), ShouldBeNil)

		type foundItem struct {
			path string
			kind string
		}
		found := []foundItem{}
		err := t.LoopItems(func(path []string, ent *Entry) error {
			found = append(found, foundItem{
				strings.Join(path, "/"),
				fmt.Sprintf("%T", ent.Etype),
			})
			if path[0] == "otherFile" {
				return errors.New("stop")
			}
			return nil
		})
		So(err, ShouldErrLike, "stop")

		So(found, ShouldResemble, []foundItem{
			{"someFile", "*toc.Entry_File"},
			{"someSymlink", "*toc.Entry_Symlink"},
			{"someTree", "*toc.Entry_Tree"},
			{"someTree/subFile", "*toc.Entry_File"},
			{"someTree/subSymlink", "*toc.Entry_Symlink"},
			{"otherFile", "*toc.Entry_File"},
		})
	})
}
