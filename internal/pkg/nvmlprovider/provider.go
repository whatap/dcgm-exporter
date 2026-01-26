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

package nvmlprovider

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type MIGDeviceInfo struct {
	ParentUUID        string
	GPUInstanceID     int
	ComputeInstanceID int
}

var nvmlInterface NVML

// Initialize sets up the Singleton NVML interface.
func Initialize() error {
	var err error
	nvmlInterface, err = newNVMLProvider()
	if err != nil {
		return err
	}
	return nil
}

// reset clears the current NVML interface instance.
func reset() {
	nvmlInterface = nil
}

// Client retrieves the current NVML interface instance.
func Client() NVML {
	return nvmlInterface
}

// SetClient sets the current NVML interface instance to the provided one.
func SetClient(n NVML) {
	nvmlInterface = n
}

// nvmlProvider implements NVML Interface
type nvmlProvider struct {
	initialized bool
	migCache    map[string]*MIGDeviceInfo
	lock        sync.RWMutex
}

func newNVMLProvider() (NVML, error) {
	// Check if a NVML client already exists and return it if so.
	if Client() != nil {
		if p, ok := Client().(*nvmlProvider); ok && p.initialized {
			slog.Info("NVML already initialized.")
			return Client(), nil
		}
	}

	slog.Info("Attempting to initialize NVML library.")
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		slog.Error(fmt.Sprintf("Cannot init NVML library; err: %v", err))
		return &nvmlProvider{initialized: false}, err
	}

	return &nvmlProvider{
		initialized: true,
		migCache:    make(map[string]*MIGDeviceInfo),
	}, nil
}

func (n *nvmlProvider) preCheck() error {
	if !n.initialized {
		return fmt.Errorf("NVML library not initialized")
	}

	return nil
}

// GetMIGDeviceInfoByID returns information about MIG DEVICE by ID
func (n *nvmlProvider) GetMIGDeviceInfoByID(uuid string) (*MIGDeviceInfo, error) {
	if err := n.preCheck(); err != nil {
		slog.Error(fmt.Sprintf("failed to get MIG Device Info; err: %v", err))
		return nil, err
	}

	n.lock.RLock()
	if info, ok := n.migCache[uuid]; ok {
		n.lock.RUnlock()
		if info == nil {
			return nil, fmt.Errorf("previously failed to get MIG device info")
		}
		return info, nil
	}
	n.lock.RUnlock()

	device, ret := nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		info, err := getMIGDeviceInfoForNewDriver(device)
		if err == nil {
			n.lock.Lock()
			n.migCache[uuid] = info
			n.lock.Unlock()
			return info, nil
		}
		n.lock.Lock()
		n.migCache[uuid] = nil
		n.lock.Unlock()
		return nil, err
	}

	info, err := getMIGDeviceInfoForOldDriver(uuid)
	if err == nil {
		n.lock.Lock()
		n.migCache[uuid] = info
		n.lock.Unlock()
		return info, nil
	}
	n.lock.Lock()
	n.migCache[uuid] = nil
	n.lock.Unlock()
	return nil, err
}

// getMIGDeviceInfoForNewDriver identifies MIG Device Information for drivers >= R470 (470.42.01+),
// each MIG device is assigned a GPU UUID starting with MIG-<UUID>.
func getMIGDeviceInfoForNewDriver(device nvml.Device) (*MIGDeviceInfo, error) {
	parentDevice, ret := device.GetDeviceHandleFromMigDeviceHandle()
	if ret != nvml.SUCCESS {
		return nil, errors.New(nvml.ErrorString(ret))
	}

	parentUUID, ret := parentDevice.GetUUID()
	if ret != nvml.SUCCESS {
		return nil, errors.New(nvml.ErrorString(ret))
	}

	gi, ret := device.GetGpuInstanceId()
	if ret != nvml.SUCCESS {
		return nil, errors.New(nvml.ErrorString(ret))
	}

	ci, ret := device.GetComputeInstanceId()
	if ret != nvml.SUCCESS {
		return nil, errors.New(nvml.ErrorString(ret))
	}

	return &MIGDeviceInfo{
		ParentUUID:        parentUUID,
		GPUInstanceID:     gi,
		ComputeInstanceID: ci,
	}, nil
}

// getMIGDeviceInfoForOldDriver identifies MIG Device Information for drivers < R470 (e.g. R450 and R460),
// each MIG device is enumerated by specifying the CI and the corresponding parent GI. The format follows this
// convention: MIG-<GPU-UUID>/<GPU instance ID>/<Compute instance ID>.
func getMIGDeviceInfoForOldDriver(uuid string) (*MIGDeviceInfo, error) {
	tokens := strings.SplitN(uuid, "-", 2)
	if len(tokens) != 2 || tokens[0] != "MIG" {
		return nil, fmt.Errorf("unable to parse '%s' as MIG device UUID", uuid)
	}

	gpuTokens := strings.SplitN(tokens[1], "/", 3)
	if len(gpuTokens) != 3 || !strings.HasPrefix(gpuTokens[0], "GPU-") {
		return nil, fmt.Errorf("invalid MIG device UUID '%s'", uuid)
	}

	gi, err := strconv.Atoi(gpuTokens[1])
	if err != nil {
		return nil, fmt.Errorf("invalid GPU instance ID '%s' for MIG device '%s'", gpuTokens[1], uuid)
	}

	ci, err := strconv.Atoi(gpuTokens[2])
	if err != nil {
		return nil, fmt.Errorf("invalid Compute instance ID '%s' for MIG device '%s'", gpuTokens[2], uuid)
	}

	return &MIGDeviceInfo{
		ParentUUID:        gpuTokens[0],
		GPUInstanceID:     gi,
		ComputeInstanceID: ci,
	}, nil
}

// GetAllGPUProcessInfo returns information about all GPU processes across all devices
func (n *nvmlProvider) GetAllGPUProcessInfo() ([]GPUProcessInfo, error) {
	if err := n.preCheck(); err != nil {
		return nil, err
	}

	var allProcesses []GPUProcessInfo

	// Get device count
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %v", nvml.ErrorString(ret))
	}

	// Process each device
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		parentUUID, ret := device.GetUUID()
		if ret != nvml.SUCCESS {
			slog.Warn("Failed to get device UUID", "index", i, "error", nvml.ErrorString(ret))
			continue
		}

		// Check for MIG mode
		isMIG := false
		mode, _, ret := device.GetMigMode()
		if ret == nvml.SUCCESS && mode == nvml.DEVICE_MIG_ENABLE {
			// Try to find MIG devices
			maxMigs, ret := device.GetMaxMigDeviceCount()
			if ret == nvml.SUCCESS {
				foundMigs := false
				for j := 0; j < maxMigs; j++ {
					migDevice, ret := device.GetMigDeviceHandleByIndex(j)
					if ret != nvml.SUCCESS {
						continue
					}
					foundMigs = true
					// Get processes for this MIG device
					// We pass 'i' as the parent device index.
					processes, err := getDeviceProcesses(migDevice, i, parentUUID)
					if err == nil {
						allProcesses = append(allProcesses, processes...)
					}
				}
				if foundMigs {
					isMIG = true
				}
			}
		}

		// If not MIG or no MIG devices found, check the device itself
		if !isMIG {
			processes, err := getDeviceProcesses(device, i, parentUUID)
			if err != nil {
				continue
			}
			allProcesses = append(allProcesses, processes...)
		}
	}

	return allProcesses, nil
}

func (n *nvmlProvider) GetGPUUUIDs() ([]string, error) {
	if err := n.preCheck(); err != nil {
		return nil, err
	}

	var uuids []string

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %s", nvml.ErrorString(ret))
	}

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			continue
		}

		// Check for MIG mode
		isMIG := false
		mode, _, ret := device.GetMigMode()
		if ret == nvml.SUCCESS && mode == nvml.DEVICE_MIG_ENABLE {
			maxMigs, ret := device.GetMaxMigDeviceCount()
			if ret == nvml.SUCCESS {
				foundMigs := false
				for j := 0; j < maxMigs; j++ {
					migDevice, ret := device.GetMigDeviceHandleByIndex(j)
					if ret != nvml.SUCCESS {
						continue
					}
					foundMigs = true
					uuid, ret := migDevice.GetUUID()
					if ret == nvml.SUCCESS {
						uuids = append(uuids, uuid)
					}
				}
				if foundMigs {
					isMIG = true
				}
			}
		}

		if !isMIG {
			uuid, ret := device.GetUUID()
			if ret == nvml.SUCCESS {
				uuids = append(uuids, uuid)
			}
		}
	}

	return uuids, nil
}

// getDeviceProcesses retrieves all processes running on a specific GPU device
func getDeviceProcesses(device nvml.Device, gpuIndex int, parentUUID string) ([]GPUProcessInfo, error) {
	var allProcesses []GPUProcessInfo

	uuid, ret := device.GetUUID()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device uuid: %v", nvml.ErrorString(ret))
	}

	// Get compute processes (Type C)
	computeProcesses, ret := device.GetComputeRunningProcesses()
	if ret == nvml.SUCCESS {
		for _, proc := range computeProcesses {
			info := GPUProcessInfo{
				Device:     gpuIndex,
				PID:        proc.Pid,
				Type:       "C",
				Command:    getProcessName(proc.Pid),
				UUID:       uuid,
				ParentUUID: parentUUID,
			}
			allProcesses = append(allProcesses, info)
		}
	}

	// Get graphics processes (Type G)
	graphicsProcesses, ret := device.GetGraphicsRunningProcesses()
	if ret == nvml.SUCCESS {
		for _, proc := range graphicsProcesses {
			info := GPUProcessInfo{
				Device:     gpuIndex,
				PID:        proc.Pid,
				Type:       "G",
				Command:    getProcessName(proc.Pid),
				UUID:       uuid,
				ParentUUID: parentUUID,
			}
			allProcesses = append(allProcesses, info)
		}
	}

	return allProcesses, nil
}

// getProcessName retrieves the full process command path from PID
func getProcessName(pid uint32) string {
	// Try to read from /proc/<pid>/cmdline first (full command line) - Linux
	cmdlinePath := fmt.Sprintf("/proc/%d/cmdline", pid)
	if data, err := os.ReadFile(cmdlinePath); err == nil {
		// cmdline uses null bytes as separators, so we take the first part
		cmdline := string(data)
		if idx := strings.Index(cmdline, "\x00"); idx > 0 {
			cmdline = cmdline[:idx]
		}
		// Return the full command path (like /Xwayland)
		if cmdline != "" {
			return cmdline
		}
	}

	// Fallback to /proc/<pid>/comm (process name only) - Linux
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	if data, err := os.ReadFile(commPath); err == nil {
		return strings.TrimSpace(string(data))
	}

	return "unknown"
}

// Cleanup performs cleanup operations for the NVML provider
func (n *nvmlProvider) Cleanup() {
	if err := n.preCheck(); err == nil {
		reset()
	}
}
