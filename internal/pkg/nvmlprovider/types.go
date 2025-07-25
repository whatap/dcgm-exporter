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

//go:generate go run -v go.uber.org/mock/mockgen  -destination=../../mocks/pkg/nvmlprovider/mock_client.go -package=nvmlprovider -copyright_file=../../../hack/header.txt . NVML

package nvmlprovider

type NVML interface {
	GetMIGDeviceInfoByID(string) (*MIGDeviceInfo, error)
	GetAllGPUProcessInfo() ([]GPUProcessInfo, error)
	Cleanup()
}

type GPUProcessInfo struct {
	Device               int     `json:"device"`
	PID                  uint32  `json:"pid"`
	Command              string  `json:"command"`
	Type                 string  `json:"type"`
	MemoryMB             uint64  `json:"memory_mb"`
	Utilization          uint32  `json:"utilization"`
	FBUsedPercent        float64 `json:"fb_used_percent"`
	UUID                 string  `json:"uuid"`
	DCGM_FI_DEV_UUID     string  `json:"dcgm_fi_dev_uuid"`
	DCGM_FI_DEV_MIG_MODE uint32  `json:"dcgm_fi_dev_mig_mode"`
}
