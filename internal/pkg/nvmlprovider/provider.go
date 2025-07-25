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
	"os/exec"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

type MIGDeviceInfo struct {
	ParentUUID        string
	GPUInstanceID     int
	ComputeInstanceID int
}

var nvmlInterface NVML

// Initialize sets up the Singleton NVML interface.
func Initialize() {
	nvmlInterface = newNVMLProvider()
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
}

func newNVMLProvider() NVML {
	// Check if a NVML client already exists and return it if so.
	if Client() != nil && Client().(nvmlProvider).initialized {
		slog.Info("NVML already initialized.")
		return Client()
	}

	slog.Info("Attempting to initialize NVML library.")
	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		err := errors.New(nvml.ErrorString(ret))
		slog.Error(fmt.Sprintf("Cannot init NVML library; err: %v", err))
		return nvmlProvider{initialized: false}
	}

	return nvmlProvider{initialized: true}
}

func (n nvmlProvider) preCheck() error {
	if !n.initialized {
		return fmt.Errorf("NVML library not initialized")
	}

	return nil
}

// GetMIGDeviceInfoByID returns information about MIG DEVICE by ID
func (n nvmlProvider) GetMIGDeviceInfoByID(uuid string) (*MIGDeviceInfo, error) {
	if err := n.preCheck(); err != nil {
		slog.Error(fmt.Sprintf("failed to get MIG Device Info; err: %v", err))
		return nil, err
	}

	device, ret := nvml.DeviceGetHandleByUUID(uuid)
	if ret == nvml.SUCCESS {
		return getMIGDeviceInfoForNewDriver(device)
	}

	return getMIGDeviceInfoForOldDriver(uuid)
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
func (n nvmlProvider) GetAllGPUProcessInfo() ([]GPUProcessInfo, error) {
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

		processes, err := getDeviceProcesses(device, i)
		if err != nil {
			continue
		}

		allProcesses = append(allProcesses, processes...)
	}

	return allProcesses, nil
}

// getDeviceProcesses retrieves all processes running on a specific GPU device
func getDeviceProcesses(device nvml.Device, gpuIndex int) ([]GPUProcessInfo, error) {
	var allProcesses []GPUProcessInfo

	// Get device utilization rates
	deviceUtilization, err := getDeviceUtilization(device)
	if err != nil {
		// If we can't get device utilization, use fallback method
		deviceUtilization = &DeviceUtilization{GPU: 0, Memory: 0}
	}

	// Get total GPU memory for FB usage percentage calculation
	totalMemory, ret := device.GetMemoryInfo()
	var totalMemoryMB uint64 = 0
	if ret == nvml.SUCCESS {
		totalMemoryMB = totalMemory.Total / (1024 * 1024)
	}

	// Get device UUID
	deviceUUID, ret := device.GetUUID()
	var uuid string = "unknown"
	if ret == nvml.SUCCESS {
		uuid = deviceUUID
	}

	// Get MIG mode - for DCGM_FI_DEV_UUID and DCGM_FI_DEV_MIG_MODE
	migMode, _, ret := device.GetMigMode()
	var migModeValue uint32 = 0
	var dcgmFiDevUUID string = uuid // Default to device UUID
	if ret == nvml.SUCCESS {
		if migMode == nvml.DEVICE_MIG_ENABLE {
			migModeValue = 1
			// For MIG devices, DCGM_FI_DEV_UUID might be different
			// In MIG mode, we still use the device UUID as DCGM_FI_DEV_UUID
			// Individual MIG instances would have their own UUIDs, but this is device-level
			dcgmFiDevUUID = uuid
		} else {
			migModeValue = 0
			dcgmFiDevUUID = uuid
		}
	} else {
		// If we can't get MIG mode, assume non-MIG
		migModeValue = 0
		dcgmFiDevUUID = uuid
	}

	// Get compute processes (Type C)
	computeProcesses, ret := device.GetComputeRunningProcesses()
	if ret == nvml.SUCCESS {
		for _, proc := range computeProcesses {
			memoryMB := proc.UsedGpuMemory / (1024 * 1024)
			utilization := calculateProcessUtilization(memoryMB, "C", deviceUtilization, len(computeProcesses))

			// Calculate FB used percentage
			var fbUsedPercent float64 = 0.0
			if totalMemoryMB > 0 {
				fbUsedPercent = (float64(memoryMB) / float64(totalMemoryMB)) * 100.0
			}

			allProcesses = append(allProcesses, GPUProcessInfo{
				Device:               gpuIndex,
				PID:                  proc.Pid,
				Type:                 "C",
				Command:              getProcessName(proc.Pid),
				MemoryMB:             memoryMB,
				Utilization:          utilization,
				FBUsedPercent:        fbUsedPercent,
				UUID:                 uuid,
				DCGM_FI_DEV_UUID:     dcgmFiDevUUID,
				DCGM_FI_DEV_MIG_MODE: migModeValue,
			})
		}
	}

	// Get graphics processes (Type G)
	graphicsProcesses, ret := device.GetGraphicsRunningProcesses()
	if ret == nvml.SUCCESS {
		for _, proc := range graphicsProcesses {
			memoryMB := proc.UsedGpuMemory / (1024 * 1024)
			utilization := calculateProcessUtilization(memoryMB, "G", deviceUtilization, len(graphicsProcesses))

			// Calculate FB used percentage
			var fbUsedPercent float64 = 0.0
			if totalMemoryMB > 0 {
				fbUsedPercent = (float64(memoryMB) / float64(totalMemoryMB)) * 100.0
			}

			allProcesses = append(allProcesses, GPUProcessInfo{
				Device:               gpuIndex,
				PID:                  proc.Pid,
				Type:                 "G",
				Command:              getProcessName(proc.Pid),
				MemoryMB:             memoryMB,
				Utilization:          utilization,
				FBUsedPercent:        fbUsedPercent,
				UUID:                 uuid,
				DCGM_FI_DEV_UUID:     dcgmFiDevUUID,
				DCGM_FI_DEV_MIG_MODE: migModeValue,
			})
		}
	}

	return allProcesses, nil
}

// DeviceUtilization represents GPU device utilization rates
type DeviceUtilization struct {
	GPU    uint32 // GPU utilization percentage
	Memory uint32 // Memory utilization percentage
}

// getDeviceUtilization retrieves device-level utilization rates using NVML API
func getDeviceUtilization(device nvml.Device) (*DeviceUtilization, error) {
	// Use NVML GetUtilizationRates API (similar to nvitop's nvmlDeviceGetUtilizationRates)
	utilization, ret := device.GetUtilizationRates()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device utilization: %v", nvml.ErrorString(ret))
	}

	return &DeviceUtilization{
		GPU:    utilization.Gpu,
		Memory: utilization.Memory,
	}, nil
}

// calculateProcessUtilization calculates process-level utilization based on device utilization and memory usage
func calculateProcessUtilization(memoryMB uint64, processType string, deviceUtil *DeviceUtilization, processCount int) uint32 {
	// If no device utilization available, fall back to memory-based estimation
	if deviceUtil.GPU == 0 && deviceUtil.Memory == 0 {
		return calculateMemoryBasedUtilization(memoryMB, processType)
	}

	// For compute processes, use GPU utilization as base
	if processType == "C" {
		if processCount == 0 {
			return 0
		}

		// Distribute GPU utilization among compute processes based on memory usage
		// This is a heuristic approach since NVML doesn't provide per-process GPU utilization
		baseUtilization := deviceUtil.GPU

		// Weight by memory usage (processes with more memory get higher utilization)
		if memoryMB > 1024 {
			return min(baseUtilization, 100) // High memory usage gets full share
		} else if memoryMB > 512 {
			return min(baseUtilization*80/100, 100) // Medium memory usage gets 80%
		} else {
			return min(baseUtilization*50/100, 100) // Low memory usage gets 50%
		}
	}

	// For graphics processes, use a portion of GPU utilization
	if processType == "G" {
		if processCount == 0 {
			return 0
		}

		// Graphics processes typically use less GPU compute
		baseUtilization := deviceUtil.GPU / 2 // Use half of device utilization as base

		if memoryMB > 512 {
			return min(baseUtilization, 100)
		} else {
			return min(baseUtilization*60/100, 100)
		}
	}

	return 0
}

// calculateMemoryBasedUtilization provides fallback utilization calculation based on memory usage
func calculateMemoryBasedUtilization(memoryMB uint64, processType string) uint32 {
	// Fallback method when device utilization is not available
	if memoryMB == 0 {
		return 0
	}

	if processType == "C" {
		if memoryMB >= 1024 {
			return 85 // High utilization for compute processes with >1GB memory
		} else if memoryMB >= 512 {
			return 60 // Medium utilization for 512MB-1GB memory
		} else {
			return 25 // Low utilization for <512MB memory
		}
	}

	if processType == "G" {
		if memoryMB >= 1024 {
			return 70 // High utilization for graphics processes with >1GB memory
		} else if memoryMB >= 256 {
			return 45 // Medium utilization for 256MB-1GB memory
		} else {
			return 15 // Low utilization for <256MB memory
		}
	}

	return 50 // Default fallback
}

// min returns the minimum of two uint32 values
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
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

	// Fallback to ps command for macOS and other systems
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "comm=")
	if output, err := cmd.Output(); err == nil {
		processName := strings.TrimSpace(string(output))
		if processName != "" {
			// Return the full command path without stripping directory
			return processName
		}
	}

	return "unknown"
}

// Cleanup performs cleanup operations for the NVML provider
func (n nvmlProvider) Cleanup() {
	if err := n.preCheck(); err == nil {
		reset()
	}
}
