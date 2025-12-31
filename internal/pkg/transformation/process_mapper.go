package transformation

import (
	"path/filepath"
	"strconv"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/collector"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/deviceinfo"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/nvmlprovider"
)

type ProcessMapper struct{}

func NewProcessMapper() *ProcessMapper {
	return &ProcessMapper{}
}

func (t *ProcessMapper) Name() string {
	return "ProcessMapper"
}

func (t *ProcessMapper) Process(metrics collector.MetricsByCounter, _ deviceinfo.Provider) error {
	// 1. Get current process info from NVML
	// We ignore error here to allow running without NVML process info if it fails transiently
	processes, err := nvmlprovider.Client().GetAllGPUProcessInfo()
	if err != nil {
		return nil
	}

	// 2. Index processes by GPU UUID
	procMap := make(map[string][]nvmlprovider.GPUProcessInfo)
	for _, p := range processes {
		if p.UUID != "" {
			procMap[p.UUID] = append(procMap[p.UUID], p)
		}
		// Also index by ParentUUID (Physical GPU) to support physical metrics (like WeightedUtil) matching MIG processes
		if p.ParentUUID != "" && p.ParentUUID != p.UUID {
			procMap[p.ParentUUID] = append(procMap[p.ParentUUID], p)
		}
	}

	// 3. Iterate over metrics and enrich
	for counter, metricList := range metrics {
		var newMetrics []collector.Metric

		for _, m := range metricList {
			// Find processes for this GPU by UUID
			// Metric.GPUUUID should match the UUID collected from NVML (including MIG UUIDs)
			procs, ok := procMap[m.GPUUUID]
			if !ok || len(procs) == 0 {
				// No processes found, keep original metric
				newMetrics = append(newMetrics, m)
				continue
			}

			// If processes found, duplicate metric for each process
			for _, p := range procs {
				mCopy := m
				if mCopy.Attributes == nil {
					mCopy.Attributes = make(map[string]string)
				} else {
					// Deep copy map
					newAttrs := make(map[string]string)
					for k, v := range m.Attributes {
						newAttrs[k] = v
					}
					mCopy.Attributes = newAttrs
				}

				mCopy.Attributes["pid"] = strconv.FormatUint(uint64(p.PID), 10)
				mCopy.Attributes["command"] = p.Command
				mCopy.Attributes["process_name"] = filepath.Base(p.Command)
				mCopy.Attributes["type"] = p.Type

				newMetrics = append(newMetrics, mCopy)
			}
		}

		metrics[counter] = newMetrics
	}

	return nil
}
