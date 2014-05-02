// Copyright 2013 Google Inc. All Rights Reserved.
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

package commands

import (
	"fmt"
	"io/ioutil"
	"os"
	gopath "path"
	"strings"

	"github.com/haruyama/drive/config"
	"github.com/haruyama/drive/remote"
	"github.com/haruyama/drive/types"
)

// Pushes to remote if local path exists and in a god context. If path is a
// directory, it recursively pushes to the remote if there are local changes.
// It doesn't check if there are local changes if isForce is set.
func (g *Commands) Push() (err error) {
	absPath := g.context.AbsPathOf(g.opts.Path)
	r, err := g.rem.FindByPath(g.opts.Path)
	if err != nil && err != remote.ErrPathNotExists {
		return err
	}

	var l *types.File
	localinfo, _ := os.Stat(absPath)
	if localinfo != nil {
		l = types.NewLocalFile(absPath, localinfo)
	}

	fmt.Println("Resolving...")
	var cl []*types.Change
	if cl, err = g.resolveChangeListRecv(true, g.opts.Path, r, l); err != nil {
		return err
	}

	if ok := printChangeList(cl, g.opts.IsNoPrompt); ok {
		return g.playPushChangeList(cl)
	}
	return
}

func (g *Commands) playPushChangeList(cl []*types.Change) (err error) {
	g.taskStart(len(cl))
	for _, c := range cl {
		switch c.Op() {
		case types.OpMod:
			g.remoteMod(c)
		case types.OpAdd:
			g.remoteAdd(c)
		case types.OpDelete:
			g.remoteDelete(c)
		}
	}
	g.taskFinish()
	return err
}

func (g *Commands) remoteMod(change *types.Change) (err error) {
	defer g.taskDone()
	absPath := g.context.AbsPathOf(change.Path)
	var parent *types.File
	if change.Dest != nil {
		change.Src.Id = change.Dest.Id // TODO: bad hack
	}

	p := strings.Split(change.Path, "/")
	p = append([]string{"/"}, p[:len(p)-1]...)
	if parent, err = g.rem.FindByPath(gopath.Join(p...)); err != nil {
		return
	}

	var body *os.File
	if !change.Src.IsDir {
		// TODO: handle errors, read more efficiently for large files
		body, _ = os.Open(absPath)
		defer body.Close()
	}
	if _, err = g.rem.Upsert(parent.Id, change.Src, body); err != nil {
		return
	}
	return
}

func (g *Commands) remoteAdd(change *types.Change) (err error) {
	return g.remoteMod(change)
}

func (g *Commands) remoteDelete(change *types.Change) (err error) {
	defer g.taskDone()
	return g.rem.Trash(change.Dest.Id)
}

func list(context *config.Context, path string) (files []*types.File, err error) {
	absPath := context.AbsPathOf(path)
	var f []os.FileInfo
	if f, err = ioutil.ReadDir(absPath); err != nil {
		return
	}
	for _, file := range f {
		// ignore hidden files
		if !strings.HasPrefix(file.Name(), ".") {
			files = append(files, types.NewLocalFile(gopath.Join(absPath, file.Name()), file))
		}
	}
	return
}
