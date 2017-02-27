// Copyright 2017 Robert Iannucci Jr. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package toc

import (
	"regexp"
	"strings"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
)

// LoopItems does a depth-first traversal of the TOC, invoking cb for every
// Entry encountered.
//
// This uses a stack-based (non-recursive) implementation.
//
// LoopItems never returns an error by itself, but will forward the error
// returned by `cb` (if any).  Returning an error from cb immediately stops the
// loop.
//
// If cb needs to retain the path slice, it should make a copy. Modifying the
// path slice is undefined.
func (t *TOC) LoopItems(cb func(path []string, ent *Entry) error) error {
	path := []string{}

	// nodes stores the stack of Tree objects we've encountered so far.
	nodes := []*Tree{t.Root}
	// indexes stores the index for each Tree.Entries, so that we can resume
	// where we left off when we pop the stack.
	indexes := []int{0}

	pop := func() (*Tree, int) {
		j := len(nodes) - 1
		n, i := nodes[j], indexes[j]
		nodes, indexes = nodes[:j], indexes[:j]
		return n, i
	}

	push := func(n *Tree, i int) {
		nodes, indexes = append(nodes, n), append(indexes, i)
	}

	for len(nodes) > 0 {
		curNode, curIdx := pop()

		for i := curIdx; i < len(curNode.Entries); i++ {
			e := curNode.Entries[i]

			path = append(path[:len(nodes)], e.Name)

			if err := cb(path, e); err != nil {
				return err
			}

			if t := e.GetTree(); t != nil {
				push(curNode, i+1)
				push(t, 0)
				break
			}
		}
	}

	return nil
}

func (t *TOC) Validate() error {
	return t.Root.Validate(t.CaseSafe, -1)
}

func (t *Tree) Validate(caseSafe bool, depth int) error {
	var lowerNames stringset.Set
	if caseSafe {
		lowerNames = stringset.New(len(t.GetEntries()))
	}
	names := stringset.New(len(t.GetEntries()))
	for _, entry := range t.GetEntries() {
		if !names.Add(entry.Name) {
			return errors.Reason("duplicate entry %(name)q").D("name", entry.Name).Err()
		}
		if caseSafe && !lowerNames.Add(strings.ToLower(entry.Name)) {
			return errors.Reason("case-sensitive entry %(name)q").
				D("name", entry.Name).Err()
		}
		if err := entry.Validate(caseSafe, depth+1); err != nil {
			return errors.Annotate(err).Reason("in entry %(name)q").
				D("name", entry.Name).Err()
		}
	}
	return nil
}

var badChars = regexp.MustCompile("[<>:\"/\\|?*\x00-\x1f]")

func checkPathPiece(piece string, allowRel bool) error {
	if piece == "" {
		return errors.New("empty path component")
	}
	if piece == "." {
		return errors.New("'.' path component")
	}
	if idxs := badChars.FindStringIndex(piece); len(idxs) > 0 {
		return errors.Reason("bad char %(char)q in path component").
			D("char", piece[idxs[0]:idxs[1]]).Err()
	}
	if !allowRel {
		if piece == ".." {
			return errors.Reason("relative path segment %(piece)q not allowed").
				D("piece", piece).Err()
		}
	}
	return nil
}

func (e *Entry) Validate(caseSafe bool, depth int) error {
	if err := checkPathPiece(e.Name, false); err != nil {
		return err
	}

	switch ent := e.Etype.(type) {
	case *Entry_File:
		return ent.File.Validate()
	case *Entry_Tree:
		return ent.Tree.Validate(caseSafe, depth)
	case *Entry_Symlink:
		return ent.Symlink.Validate(depth)
	}
	panic("impossible")
}

func (s *SymLink) Validate(depth int) error {
	if len(s.Target) == 0 {
		return errors.New("empty symlink target")
	}

	level := 0
	for i, p := range s.Target {
		if err := checkPathPiece(p, true); err != nil {
			return errors.Annotate(err).Reason("symlink target piece %(i)d").
				D("i", i).Err()
		}
		if p == ".." {
			level++
			if level > depth {
				return errors.Reason("symlink target %(target)q escapes root").
					D("target", s.Target).Err()
			}
		}
	}
	return nil
}

func (f *File) Validate() error {
	return nil
}
