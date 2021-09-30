// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"bytes"
	"fmt"
)

// exported from runtime
func modinfo() string

// ReadBuildInfo returns the build information embedded
// in the running binary. The information is available only
// in binaries built with module support.
func ReadBuildInfo() (info *BuildInfo, ok bool) {
	data := modinfo()
	if len(data) < 32 {
		return nil, false
	}
	data = data[16 : len(data)-16]
	bi := &BuildInfo{}
	if err := bi.UnmarshalText([]byte(data)); err != nil {
		return nil, false
	}
	return bi, true
}

// BuildInfo represents the build information read from a Go binary.
type BuildInfo struct {
	Path string    // The main package path
	Main Module    // The module containing the main package
	Deps []*Module // Module dependencies
}

// Module represents a module.
type Module struct {
	Path    string  // module path
	Version string  // module version
	Sum     string  // checksum
	Replace *Module // replaced by this module
}

func (bi *BuildInfo) MarshalText() ([]byte, error) {
	buf := &bytes.Buffer{}
	if bi.Path != "" {
		fmt.Fprintf(buf, "path\t%s\n", bi.Path)
	}
	var formatMod func(string, Module)
	formatMod = func(word string, m Module) {
		buf.WriteString(word)
		buf.WriteByte('\t')
		buf.WriteString(m.Path)
		mv := m.Version
		if mv == "" {
			mv = "(devel)"
		}
		buf.WriteByte('\t')
		buf.WriteString(mv)
		if m.Replace == nil {
			buf.WriteByte('\t')
			buf.WriteString(m.Sum)
		} else {
			buf.WriteByte('\n')
			formatMod("=>", *m.Replace)
		}
		buf.WriteByte('\n')
	}
	if bi.Main.Path != "" {
		formatMod("mod", bi.Main)
	}
	for _, dep := range bi.Deps {
		formatMod("dep", *dep)
	}

	return buf.Bytes(), nil
}

func (bi *BuildInfo) UnmarshalText(data []byte) (err error) {
	*bi = BuildInfo{}
	lineNum := 1
	defer func() {
		if err != nil {
			err = fmt.Errorf("could not parse Go build info: line %d: %w", lineNum, err)
		}
	}()

	var (
		pathLine = []byte("path\t")
		modLine  = []byte("mod\t")
		depLine  = []byte("dep\t")
		repLine  = []byte("=>\t")
		newline  = []byte("\n")
		tab      = []byte("\t")
	)

	readModuleLine := func(elem [][]byte) (Module, error) {
		if len(elem) != 2 && len(elem) != 3 {
			return Module{}, fmt.Errorf("expected 2 or 3 columns; got %d", len(elem))
		}
		sum := ""
		if len(elem) == 3 {
			sum = string(elem[2])
		}
		return Module{
			Path:    string(elem[0]),
			Version: string(elem[1]),
			Sum:     sum,
		}, nil
	}

	var (
		last *Module
		line []byte
		ok   bool
	)
	// Reverse of BuildInfo.String()
	for len(data) > 0 {
		line, data, ok = bytes.Cut(data, newline)
		if !ok {
			break
		}
		switch {
		case bytes.HasPrefix(line, pathLine):
			elem := line[len(pathLine):]
			bi.Path = string(elem)
		case bytes.HasPrefix(line, modLine):
			elem := bytes.Split(line[len(modLine):], tab)
			last = &bi.Main
			*last, err = readModuleLine(elem)
			if err != nil {
				return err
			}
		case bytes.HasPrefix(line, depLine):
			elem := bytes.Split(line[len(depLine):], tab)
			last = new(Module)
			bi.Deps = append(bi.Deps, last)
			*last, err = readModuleLine(elem)
			if err != nil {
				return err
			}
		case bytes.HasPrefix(line, repLine):
			elem := bytes.Split(line[len(repLine):], tab)
			if len(elem) != 3 {
				return fmt.Errorf("expected 3 columns for replacement; got %d", len(elem))
			}
			if last == nil {
				return fmt.Errorf("replacement with no module on previous line")
			}
			last.Replace = &Module{
				Path:    string(elem[0]),
				Version: string(elem[1]),
				Sum:     string(elem[2]),
			}
			last = nil
		}
		lineNum++
	}
	return nil
}
