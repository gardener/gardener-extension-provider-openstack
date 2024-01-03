// Copyright 2022 IronCore authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package link

import (
	"fmt"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ironcore-dev/vgopath/internal/module"
	"github.com/spf13/pflag"
)

type Node struct {
	Segment  string
	Module   *module.Module
	Children []Node
}

func insertModuleInNode(node *Node, mod module.Module, relativeSegments []string) error {
	if len(relativeSegments) == 0 {
		if node.Module != nil {
			return fmt.Errorf("cannot insert module %s into node %s: module %s already exists", mod.Path, node.Segment, node.Module.Path)
		}

		node.Module = &mod
		return nil
	}

	var (
		idx     = -1
		segment = relativeSegments[0]
	)
	for i, child := range node.Children {
		if child.Segment == segment {
			idx = i
			break
		}
	}

	var child *Node
	if idx == -1 {
		child = &Node{Segment: segment}
	} else {
		child = &node.Children[idx]
	}

	if err := insertModuleInNode(child, mod, relativeSegments[1:]); err != nil {
		return err
	}

	if idx == -1 {
		node.Children = append(node.Children, *child)
	}

	return nil
}

func BuildModuleNodes(modules []module.Module) ([]Node, error) {
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	nodeByRootSegment := make(map[string]*Node)

	for _, mod := range modules {
		if mod.Path == "" {
			return nil, fmt.Errorf("invalid empty module path")
		}

		segments := strings.Split(mod.Path, "/")

		rootSegment := segments[0]
		node, ok := nodeByRootSegment[rootSegment]
		if !ok {
			node = &Node{Segment: rootSegment}
			nodeByRootSegment[rootSegment] = node
		}

		if err := insertModuleInNode(node, mod, segments[1:]); err != nil {
			return nil, err
		}
	}

	res := make([]Node, 0, len(nodeByRootSegment))
	for _, node := range nodeByRootSegment {
		res = append(res, *node)
	}
	return res, nil
}

func FilterModulesWithoutDir(modules []module.Module) []module.Module {
	var res []module.Module
	for _, mod := range modules {
		// Don't vendor modules without backing directory.
		if mod.Dir == "" {
			continue
		}

		res = append(res, mod)
	}

	return res
}

type Options struct {
	SrcDir    string
	SkipGoBin bool
	SkipGoSrc bool
	SkipGoPkg bool
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.SrcDir, "src-dir", o.SrcDir, "Source directory for linking. Empty string indicates current directory.")
	fs.BoolVar(&o.SkipGoPkg, "skip-go-pkg", o.SkipGoPkg, "Whether to skip mirroring $GOPATH/pkg")
	fs.BoolVar(&o.SkipGoBin, "skip-go-bin", o.SkipGoBin, "Whether to skip mirroring $GOBIN")
	fs.BoolVar(&o.SkipGoSrc, "skip-go-src", o.SkipGoSrc, "Whether to skip mirroring modules as src")
}

func Link(dstDir string, opts Options) error {
	if opts.SrcDir == "" {
		opts.SrcDir = "."
	}

	if !opts.SkipGoSrc {
		if err := GoSrc(dstDir, opts.SrcDir); err != nil {
			return fmt.Errorf("error linking GOPATH/src: %w", err)
		}
	}

	if !opts.SkipGoBin {
		if err := GoBin(dstDir); err != nil {
			return fmt.Errorf("error linking GOPATH/bin: %w", err)
		}
	}

	if !opts.SkipGoPkg {
		if err := GoPkg(dstDir); err != nil {
			return fmt.Errorf("error linking GOPATH/pkg: %w", err)
		}
	}

	return nil
}

func GoBin(dstDir string) error {
	dstGoBinDir := filepath.Join(dstDir, "bin")
	if err := os.RemoveAll(dstGoBinDir); err != nil {
		return err
	}

	srcGoBinDir := os.Getenv("GOBIN")
	if srcGoBinDir == "" {
		srcGoBinDir = filepath.Join(build.Default.GOPATH, "bin")
	}

	if err := os.Symlink(srcGoBinDir, dstGoBinDir); err != nil {
		return err
	}
	return nil
}

func GoPkg(dstDir string) error {
	dstGoPkgDir := filepath.Join(dstDir, "pkg")
	if err := os.RemoveAll(dstGoPkgDir); err != nil {
		return err
	}

	if err := os.Symlink(filepath.Join(build.Default.GOPATH, "pkg"), dstGoPkgDir); err != nil {
		return err
	}
	return nil
}

func GoSrc(dstDir, srcDir string) error {
	mods, err := module.ReadAllGoListModules(module.InDir(srcDir))
	if err != nil {
		return fmt.Errorf("error reading modules: %w", err)
	}

	mods = FilterModulesWithoutDir(mods)

	nodes, err := BuildModuleNodes(mods)
	if err != nil {
		return fmt.Errorf("error building module tree: %w", err)
	}

	dstGoSrcDir := filepath.Join(dstDir, "src")
	if err := os.RemoveAll(dstGoSrcDir); err != nil {
		return err
	}

	if err := os.Mkdir(dstGoSrcDir, 0777); err != nil {
		return err
	}

	if err := Nodes(dstGoSrcDir, nodes); err != nil {
		return err
	}
	return nil
}

type linkNodeError struct {
	path string
	err  error
}

func (l *linkNodeError) Error() string {
	return fmt.Sprintf("[path %s]: %v", l.path, l.err)
}

func joinLinkNodeError(node Node, err error) error {
	if linkNodeErr, ok := err.(*linkNodeError); ok {
		return &linkNodeError{
			path: path.Join(node.Segment, linkNodeErr.path),
			err:  linkNodeErr.err,
		}
	}
	return &linkNodeError{
		path: node.Segment,
		err:  err,
	}
}

func Nodes(dir string, nodes []Node) error {
	for _, node := range nodes {
		if err := linkNode(dir, node); err != nil {
			return joinLinkNodeError(node, err)
		}
	}
	return nil
}

func linkNode(dir string, node Node) error {
	dstDir := filepath.Join(dir, node.Segment)

	// If the node specifies a module and no children are present, we can take optimize and directly
	// symlink the module directory to the destination directory.
	if node.Module != nil && len(node.Children) == 0 {
		srcDir := node.Module.Dir

		if err := os.Symlink(srcDir, dstDir); err != nil {
			return fmt.Errorf("error symlinking node: %w", err)
		}
	}

	if err := os.RemoveAll(dstDir); err != nil {
		return err
	}

	if err := os.Mkdir(dstDir, 0777); err != nil {
		return err
	}

	if node.Module != nil {
		srcDir := node.Module.Dir
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			srcPath := filepath.Join(srcDir, entry.Name())
			dstPath := filepath.Join(dstDir, entry.Name())
			if err := os.Symlink(srcPath, dstPath); err != nil {
				return fmt.Errorf("error symlinking entry %s to %s: %w", srcPath, dstPath, err)
			}
		}
	}
	return Nodes(dstDir, node.Children)
}
