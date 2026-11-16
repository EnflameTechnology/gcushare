/*
 * Copyright (c) 2019, NVIDIA CORPORATION.  All rights reserved.
 * Modifications Copyright (c) 2020 Enflame Technologies, Inc. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package watcher

import (
	"os"
	"time"

	"gcushare-scheduler-plugin/pkg/config"
	"gcushare-scheduler-plugin/pkg/consts"
	"gcushare-scheduler-plugin/pkg/logs"

	"github.com/fsnotify/fsnotify"
)

func NewFSWatcher(files ...string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			watcher.Close()
			return nil, err
		}
	}

	return watcher, nil
}

func WatchConfig() {
	configPath := config.TopscloudPath + config.ConfigFileName
	logs.Info("Starting watch config file: %s", configPath)
	watcher, err := NewFSWatcher(config.TopscloudPath)
	if err != nil {
		logs.Error(err, "Failed to created FS watcher for path: %s", config.TopscloudPath)
		os.Exit(1)
	}
	defer watcher.Close()

	for {
		select {
		// If config file is detected to be modified, will be automatically restarted
		case event := <-watcher.Events:
			if event.Name == configPath && (event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write) {
				logs.Warn("fsnotify: config file: %s has been %v, need restart %s", event.Name, event.Op, consts.COMPONENT_NAME)
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}
		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			logs.Error(err, "fsnotify failed")
		}
	}
}
