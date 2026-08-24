/*
Copyright 2026 NVIDIA CORPORATION & AFFILIATES

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package coverage supports runtime coverage collection for integration tests.
package coverage

import (
	"os"
	"os/signal"
	"runtime/coverage"
	"syscall"

	ctrl "sigs.k8s.io/controller-runtime"
)

var flushLog = ctrl.Log.WithName("coverage")

// SetupSignalHandler registers SIGUSR1 to flush in-memory coverage counters to
// GOCOVERDIR. No-op when GOCOVERDIR is unset (normal production images).
func SetupSignalHandler() {
	dir, ok := os.LookupEnv("GOCOVERDIR")
	if !ok {
		return
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	flushLog.Info("coverage flush handler enabled", "dir", dir)
	go func() {
		for range c {
			if err := coverage.WriteCountersDir(dir); err != nil {
				flushLog.Error(err, "failed to write coverage counters", "dir", dir)
				continue
			}
			flushLog.Info("flushed coverage counters", "dir", dir)
			if os.Getenv("COVERAGE_CLEAR_AFTER_FLUSH") == "1" {
				if err := coverage.ClearCounters(); err != nil {
					flushLog.Error(err, "failed to clear coverage counters after flush")
				}
			}
		}
	}()
}
