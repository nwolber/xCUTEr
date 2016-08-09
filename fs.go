// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package xCUTEr

import (
	"io/ioutil"
	"log"
	"strings"

	"github.com/fsnotify/fsnotify"

	"golang.org/x/net/context"
)

type watcher struct {
	path string
}

func (w *watcher) watch(ctx context.Context, events chan<- fsnotify.Event) {
	files, err := ioutil.ReadDir(w.path)
	if err != nil {
		log.Println(err)
		return
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println(err)
	}
	defer fsWatcher.Close()

	fsWatcher.Add(w.path)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".job") {
			events <- fsnotify.Event{
				Name: file.Name(),
				Op:   fsnotify.Create,
			}
		}
	}

	for {
		select {
		case event := <-fsWatcher.Events:
			if strings.HasSuffix(event.Name, ".job") {
				events <- event
			}
		case err := <-fsWatcher.Errors:
			log.Println(err)
		case <-ctx.Done():
			return
		}
	}
}
