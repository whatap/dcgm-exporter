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
	migMaxSlicesID     = 1212 // dcgm.DCGM_FI_DEV_MIG_MAX_SLICES
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

	// 1. Handle Non-MIG: DCGM_FI_DEV_GPU_UTIL
	nonMig := t.computeNonMIG(metrics)
	allNewMetrics = append(allNewMetrics, nonMig...)

	// 2. Handle MIG: DCGM_FI_PROF_GR_ENGINE_ACTIVE
	mig := t.computeMIG(metrics)
	allNewMetrics = append(allNewMetrics, mig...)

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

func (t *WeightedUtil) computeNonMIG(metrics collector.MetricsByCounter) []collector.Metric {
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
		val, err := strconv.ParseFloat(m.Value, 64)
		if err != nil {
			continue
		}

		// Calculate weighted util: Util / 100
		weightedVal := val / 100.0

		newMetric := m
		newMetric.Counter = counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		newMetric.Value = strconv.FormatFloat(weightedVal, 'f', -1, 64)

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

	// Need Max Slices. Try to find DCGM_FI_DEV_MIG_MAX_SLICES
	gpuMaxSlices := make(map[string]float64)

	for c, mList := range metrics {
		if c.FieldID == migMaxSlicesID {
			for _, m := range mList {
				val, err := strconv.ParseFloat(m.Value, 64)
				if err == nil {
					gpuMaxSlices[m.GPUUUID] = val
				}
			}
			break
		}
	}

	newMetrics := make([]collector.Metric, 0, len(srcMetrics))
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

		maxSlices, ok := gpuMaxSlices[m.GPUUUID]
		if !ok {
			// Default to 7.0 if not found
			maxSlices = 7.0
			slog.Debug("DCGM_FI_DEV_MIG_MAX_SLICES not found, using default", "gpu", m.GPUUUID, "default", maxSlices)
		}

		if maxSlices == 0 {
			continue
		}

		// Weighted Util = Active * (Slices / MaxSlices)
		weightedVal := val * (slices / maxSlices)

		newMetric := m
		newMetric.Counter = counters.Counter{
			FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
			FieldName: counters.DCGMExpWeightedGPUUtil,
			PromType:  "gauge",
			Help:      "Weighted GPU Utilization",
		}
		newMetric.Value = strconv.FormatFloat(weightedVal, 'f', -1, 64)

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
