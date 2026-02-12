package transformation

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/appconfig"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/collector"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/deviceinfo"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/nvmlprovider"
)

// processRelevantPrefixes defines metric field name prefixes that are meaningful per-process.
// Only these metrics will be duplicated per process; other metrics (temperature, power, clock, etc.)
// are device-level and kept as-is to avoid metric explosion.
var processRelevantPrefixes = []string{
	"DCGM_FI_DEV_GPU_UTIL",
	"DCGM_FI_DEV_MEM_COPY_UTIL",
	"DCGM_FI_DEV_ENC_UTIL",
	"DCGM_FI_DEV_DEC_UTIL",
	"DCGM_FI_DEV_FB_FREE",
	"DCGM_FI_DEV_FB_USED",
	"DCGM_FI_DEV_FB_RESERVED",
	"DCGM_FI_PROF_GR_ENGINE_ACTIVE",
	"DCGM_FI_PROF_SM_ACTIVE",
	"DCGM_FI_PROF_SM_OCCUPANCY",
	"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
	"DCGM_FI_PROF_DRAM_ACTIVE",
}

type ProcessMapper struct {
	config *appconfig.Config
}

func NewProcessMapper(c *appconfig.Config) *ProcessMapper {
	return &ProcessMapper{
		config: c,
	}
}

func (t *ProcessMapper) Name() string {
	return "ProcessMapper"
}

// isProcessRelevant checks if a counter's field name should be duplicated per process.
func isProcessRelevant(fieldName string) bool {
	for _, prefix := range processRelevantPrefixes {
		if strings.HasPrefix(fieldName, prefix) {
			return true
		}
	}
	return false
}

func (t *ProcessMapper) Process(metrics collector.MetricsByCounter, _ deviceinfo.Provider) error {
	if !t.config.CollectProcessInfo {
		return nil
	}

	// 1. Get current process info from NVML (cached within scrape interval)
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

	// 3. Iterate over metrics and enrich only process-relevant counters
	for counter, metricList := range metrics {
		if counter.FieldName == "DCGM_FI_DEV_WEIGHTED_GPU_UTIL" {
			continue
		}

		// Skip counters that are not meaningful per-process
		if !isProcessRelevant(counter.FieldName) {
			continue
		}

		var newMetrics []collector.Metric

		for _, m := range metricList {
			// Find processes for this GPU by UUID
			// Priority:
			// 1. DCGM_FI_DEV_UUID label/attribute (Real MIG UUID for MIG metrics)
			// 2. Metric.GPUUUID (Physical UUID fallback)
			searchUUID := m.GPUUUID
			if v, ok := m.Labels["DCGM_FI_DEV_UUID"]; ok && v != "" {
				searchUUID = v
			} else if v, ok := m.Attributes["DCGM_FI_DEV_UUID"]; ok && v != "" {
				searchUUID = v
			}

			procs, ok := procMap[searchUUID]
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
