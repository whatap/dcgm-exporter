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

package transformation

import (
	"bufio"
	"fmt"
	stdos "os"
	"regexp"
	"strings"
)

var (
	// Regex to extract Pod UID from cgroup path.
	// Matches patterns like:
	// /kubepods/burstable/pod6c5475af-152e-4b40-8b43-410c55986514/
	// /kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod6c5475af-152e-4b40-8b43-410c55986514.slice/
	podUIDRegex = regexp.MustCompile(`pod([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
)

// GetPodUIDFromPID attempts to find the Kubernetes Pod UID for a given PID
// by inspecting /proc/<pid>/cgroup.
func GetPodUIDFromPID(pid uint64) (string, error) {
	cgroupPath := fmt.Sprintf("/proc/%d/cgroup", pid)
	file, err := stdos.Open(cgroupPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for lines that look like Kubernetes cgroups
		if strings.Contains(line, "kubepods") {
			matches := podUIDRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				// matches[1] is the UID
				return matches[1], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("pod UID not found in cgroup for PID %d", pid)
}
