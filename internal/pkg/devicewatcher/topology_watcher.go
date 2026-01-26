/*
 * Copyright (c) 2024, NVIDIA CORPORATION.  All rights reserved.
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

package devicewatcher

import (
	"context"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/nvmlprovider"
)

func WatchTopologyChanges(ctx context.Context, intervalSeconds int) {
	slog.Info("Starting GPU topology watcher", slog.Int("interval_seconds", intervalSeconds))

	// Get initial snapshot
	initialUUIDs, err := getGPUUUIDsWithRetry(3)
	if err != nil {
		slog.Error("Failed to get initial GPU UUIDs, self-healing might not work correctly", slog.String("error", err.Error()))
		return
	}
	slog.Info("Initial GPU topology captured", slog.Any("uuids", initialUUIDs))

	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()

	consecutiveFailures := 0
	const maxConsecutiveFailures = 3

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentUUIDs, err := nvmlprovider.Client().GetGPUUUIDs()
			if err != nil {
				consecutiveFailures++
				slog.Warn("Failed to get current GPU UUIDs",
					slog.Int("failure_count", consecutiveFailures),
					slog.String("error", err.Error()))

				if consecutiveFailures >= maxConsecutiveFailures {
					slog.Error("Too many consecutive failures getting GPU UUIDs. Initiating self-healing restart.")
					os.Exit(1)
				}
				continue
			}
			consecutiveFailures = 0

			if topologyChanged(initialUUIDs, currentUUIDs) {
				slog.Info("[GPU-Watcher] MIG configuration change detected. Initiating self-restart.",
					slog.Any("old_uuids", initialUUIDs),
					slog.Any("new_uuids", currentUUIDs))
				os.Exit(1)
			}
		}
	}
}

func getGPUUUIDsWithRetry(retries int) ([]string, error) {
	var err error
	var uuids []string
	for i := 0; i < retries; i++ {
		uuids, err = nvmlprovider.Client().GetGPUUUIDs()
		if err == nil {
			return uuids, nil
		}
		time.Sleep(1 * time.Second)
	}
	return nil, err
}

func topologyChanged(oldUUIDs, newUUIDs []string) bool {
	if len(oldUUIDs) != len(newUUIDs) {
		return true
	}

	// We need to compare the sets of UUIDs regardless of order
	oldCopy := slices.Clone(oldUUIDs)
	newCopy := slices.Clone(newUUIDs)
	slices.Sort(oldCopy)
	slices.Sort(newCopy)

	return !slices.Equal(oldCopy, newCopy)
}
