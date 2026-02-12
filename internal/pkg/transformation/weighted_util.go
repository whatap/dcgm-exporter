package transformation

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/collector"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/counters"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/deviceinfo"
)

const (
	gpuUtilID          = dcgm.DCGM_FI_DEV_GPU_UTIL
	profGrEngineActive = dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE
	// Use official DCGM field constant instead of a hardcoded magic number
	migMaxSlicesID = dcgm.DCGM_FI_DEV_MIG_MAX_SLICES
)

type WeightedUtil struct{}

func NewWeightedUtil() *WeightedUtil {
	return &WeightedUtil{}
}

func (t *WeightedUtil) Name() string {
	return "WeightedUtil"
}

func (t *WeightedUtil) Process(metrics collector.MetricsByCounter, _ deviceinfo.Provider) error {
	var allNewMetrics []collector.Metric

	// Identify H-series GPUs that have PROF_GR_ENGINE_ACTIVE available
	hSeriesGPUs := t.findHSeriesGPUs(metrics)

	// 1. Handle Non-MIG: DCGM_FI_DEV_GPU_UTIL (skip H-series GPUs that use PROF_GR_ENGINE_ACTIVE)
	nonMig := t.computeNonMIG(metrics, hSeriesGPUs)
	allNewMetrics = append(allNewMetrics, nonMig...)

	// 2. Handle MIG: DCGM_FI_PROF_GR_ENGINE_ACTIVE
	mig := t.computeMIG(metrics)
	allNewMetrics = append(allNewMetrics, mig...)

	// 3. Handle H-series non-MIG: use PROF_GR_ENGINE_ACTIVE directly
	hSeriesNonMig := t.computeHSeriesNonMIG(metrics, hSeriesGPUs)
	allNewMetrics = append(allNewMetrics, hSeriesNonMig...)

	if len(allNewMetrics) > 0 {
		c := counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		metrics[c] = allNewMetrics
	}

	return nil
}

// findHSeriesGPUs returns a set of GPU indices that are H-series GPUs
// and have DCGM_FI_PROF_GR_ENGINE_ACTIVE metrics available (non-MIG).
func (t *WeightedUtil) findHSeriesGPUs(metrics collector.MetricsByCounter) map[string]bool {
	hSeriesGPUs := make(map[string]bool)

	// First, find H-series GPUs from GPU_UTIL metrics
	for c, mList := range metrics {
		if c.FieldID == gpuUtilID {
			for _, m := range mList {
				if isHSeriesGPU(m.GPUModelName) {
					hSeriesGPUs[m.GPU] = true
				}
			}
			break
		}
	}

	if len(hSeriesGPUs) == 0 {
		return hSeriesGPUs
	}

	// Verify that PROF_GR_ENGINE_ACTIVE metrics exist for these GPUs (non-MIG entries)
	hasActiveMetric := make(map[string]bool)
	for c, mList := range metrics {
		if c.FieldID == profGrEngineActive {
			for _, m := range mList {
				if m.MigProfile == "" {
					hasActiveMetric[m.GPU] = true
				}
			}
			break
		}
	}

	// Only keep H-series GPUs that have PROF_GR_ENGINE_ACTIVE available
	for gpu := range hSeriesGPUs {
		if !hasActiveMetric[gpu] {
			delete(hSeriesGPUs, gpu)
		}
	}

	return hSeriesGPUs
}

// isHSeriesGPU checks if the GPU model name indicates an H-series GPU.
func isHSeriesGPU(modelName string) bool {
	upper := strings.ToUpper(modelName)
	hModels := []string{"H100", "H200", "H800", "H20"}
	for _, h := range hModels {
		if strings.Contains(upper, h) {
			return true
		}
	}
	return false
}

// computeHSeriesNonMIG computes weighted utilization for H-series GPUs not in MIG mode
// using DCGM_FI_PROF_GR_ENGINE_ACTIVE directly.
func (t *WeightedUtil) computeHSeriesNonMIG(metrics collector.MetricsByCounter, hSeriesGPUs map[string]bool) []collector.Metric {
	if len(hSeriesGPUs) == 0 {
		return nil
	}

	var srcMetrics []collector.Metric
	for c, m := range metrics {
		if c.FieldID == profGrEngineActive {
			srcMetrics = m
			break
		}
	}

	if len(srcMetrics) == 0 {
		return nil
	}

	newMetrics := make([]collector.Metric, 0)
	for _, m := range srcMetrics {
		// Only process non-MIG entries for H-series GPUs
		if m.MigProfile != "" {
			continue
		}
		if !hSeriesGPUs[m.GPU] {
			continue
		}

		val, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			continue
		}

		newMetric := m
		newMetric.Labels = make(map[string]string, len(m.Labels)+1)
		for k, v := range m.Labels {
			newMetric.Labels[k] = v
		}
		newMetric.Attributes = make(map[string]string, len(m.Attributes))
		for k, v := range m.Attributes {
			newMetric.Attributes[k] = v
		}

		newMetric.Counter = counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		newMetric.Value = strconv.FormatFloat(val, 'f', -1, 64)
		newMetric.Labels["calculation_method"] = "prof_gr_engine_active"
		newMetric.Labels["DCGM_FI_DEV_UUID"] = newMetric.GPUUUID

		newMetrics = append(newMetrics, newMetric)
	}

	return newMetrics
}

func (t *WeightedUtil) computeNonMIG(metrics collector.MetricsByCounter, hSeriesGPUs map[string]bool) []collector.Metric {
	var srcMetrics []collector.Metric
	for c, m := range metrics {
		if c.FieldID == gpuUtilID {
			srcMetrics = m
			break
		}
	}

	if len(srcMetrics) == 0 {
		return nil
	}

	newMetrics := make([]collector.Metric, 0, len(srcMetrics))
	for _, m := range srcMetrics {
		// Skip H-series GPUs; they will be handled by computeHSeriesNonMIG
		if hSeriesGPUs[m.GPU] {
			continue
		}

		val, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			continue
		}

		// Cap GPU_UTIL at 100 to prevent abnormal values
		if val > 100.0 {
			slog.Warn("DCGM_FI_DEV_GPU_UTIL exceeded 100, capping to 100", "gpu", m.GPU, "original_value", val)
			val = 100.0
		}

		// Calculate weighted util: Util / 100
		weightedVal := val / 100.0

		// Create a new metric with deep copy of Labels/Attributes
		newMetric := m
		newMetric.Labels = make(map[string]string, len(m.Labels)+1)
		for k, v := range m.Labels {
			newMetric.Labels[k] = v
		}
		newMetric.Attributes = make(map[string]string, len(m.Attributes))
		for k, v := range m.Attributes {
			newMetric.Attributes[k] = v
		}

		newMetric.Counter = counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		newMetric.Value = strconv.FormatFloat(weightedVal, 'f', -1, 64)
		newMetric.Labels["calculation_method"] = "direct"
		newMetric.Labels["DCGM_FI_DEV_UUID"] = newMetric.GPUUUID

		newMetrics = append(newMetrics, newMetric)
	}
	return newMetrics
}

func (t *WeightedUtil) computeMIG(metrics collector.MetricsByCounter) []collector.Metric {
	var srcMetrics []collector.Metric
	for c, m := range metrics {
		if c.FieldID == profGrEngineActive {
			srcMetrics = m
			break
		}
	}

	if len(srcMetrics) == 0 {
		return nil
	}

	// Maps keyed by GPU Index (m.GPU)
	gpuMaxSlices := make(map[string]float64)
	gpuTemplates := make(map[string]collector.Metric)

	// Find DCGM_FI_DEV_MIG_MAX_SLICES to get max slices and physical device info
	for c, mList := range metrics {
		if c.FieldID == migMaxSlicesID {
			for _, m := range mList {
				val, err := strconv.ParseFloat(m.Value, 64)
				if err == nil {
					// Use GPU index as key
					gpuMaxSlices[m.GPU] = val
					// Store metric as template for physical device labels
					gpuTemplates[m.GPU] = m
				}
			}
			break
		}
	}

	// Aggregate weighted utilization per Physical GPU
	gpuWeightedSum := make(map[string]float64)

	for _, m := range srcMetrics {
		val, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			continue
		}

		// Parse Slice count from MigProfile
		slices := t.getSlicesFromProfile(m.MigProfile)
		if slices == 0.0 {
			continue
		}

		// Find parent GPU's max slices using GPU index
		maxSlices, ok := gpuMaxSlices[m.GPU]
		if !ok {
			// Fallback: If MAX_SLICES not found for this GPU index,
			// try to assume it matches if we only have one GPU or check other logic?
			// For now, default to 7.0 and log debug if we can't match.
			// But critically, we need a template for the physical GPU labels.
			// If we don't have maxSlices metric, we might not have a template.
			maxSlices = 7.0
			slog.Debug("DCGM_FI_DEV_MIG_MAX_SLICES not found for GPU, using default", "gpu", m.GPU, "default", maxSlices)
		}

		if maxSlices == 0 {
			continue
		}

		// Weighted Util = Active * (Slices / MaxSlices)
		weightedVal := val * (slices / maxSlices)

		// Accumulate
		gpuWeightedSum[m.GPU] += weightedVal
	}

	newMetrics := make([]collector.Metric, 0, len(gpuWeightedSum))
	for gpuIdx, sumVal := range gpuWeightedSum {
		// Create new metric based on template
		template, ok := gpuTemplates[gpuIdx]
		var newMetric collector.Metric

		if ok {
			newMetric = template
			// Deep copy labels/attributes
			newMetric.Labels = make(map[string]string, len(template.Labels)+1)
			for k, v := range template.Labels {
				newMetric.Labels[k] = v
			}
			newMetric.Attributes = make(map[string]string, len(template.Attributes))
			for k, v := range template.Attributes {
				newMetric.Attributes[k] = v
			}
		} else {
			// If no template (MAX_SLICES missing), we must construct best-effort metric.
			// We can pick one of the source metrics but strip MIG labels.
			// Let's find first source metric with this GPU index
			for _, m := range srcMetrics {
				if m.GPU == gpuIdx {
					newMetric = m

					// Deep copy labels/attributes to avoid polluting source and to remove MIG labels safely
					newMetric.Labels = make(map[string]string, len(m.Labels)+1)
					for k, v := range m.Labels {
						newMetric.Labels[k] = v
					}
					newMetric.Attributes = make(map[string]string, len(m.Attributes))
					for k, v := range m.Attributes {
						newMetric.Attributes[k] = v
					}

					// Clear MIG specific fields/labels
					newMetric.MigProfile = ""
					newMetric.GPUInstanceID = ""
					break
				}
			}
		}

		newMetric.Counter = counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		newMetric.Value = strconv.FormatFloat(sumVal, 'f', -1, 64)

		// Set calculation method
		newMetric.Labels["calculation_method"] = "weighted_sum"
		newMetric.Labels["DCGM_FI_DEV_UUID"] = newMetric.GPUUUID

		newMetrics = append(newMetrics, newMetric)
	}

	return newMetrics
}

func (t *WeightedUtil) getSlicesFromProfile(profile string) float64 {
	if strings.HasPrefix(profile, "1g.") {
		return 1.0
	}
	if strings.HasPrefix(profile, "2g.") {
		return 2.0
	}
	if strings.HasPrefix(profile, "3g.") {
		return 3.0
	}
	if strings.HasPrefix(profile, "4g.") {
		return 4.0
	}
	if strings.HasPrefix(profile, "7g.") {
		return 7.0
	}

	// Generic parsing: "Ng.Mgb"
	parts := strings.Split(profile, "g.")
	if len(parts) > 0 {
		if s, err := strconv.ParseFloat(parts[0], 64); err == nil {
			return s
		}
	}

	return 0.0
}
