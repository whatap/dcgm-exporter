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

package collector

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/counters"
	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
)

// calculateWeightedGPUUtil calculates weighted GPU utilization for all GPUs
func (c *DCGMCollector) calculateWeightedGPUUtil(metrics MetricsByCounter) {
	// Group metrics by GPU UUID to process each GPU separately
	gpuMetrics := make(map[string][]Metric)

	// Collect all relevant metrics grouped by GPU UUID
	for _, counterMetrics := range metrics {
		for _, metric := range counterMetrics {
			if metric.GPUUUID != "" {
				gpuMetrics[metric.GPUUUID] = append(gpuMetrics[metric.GPUUUID], metric)
			}
		}
	}

	// Process each GPU
	for gpuUUID, gpuMetricList := range gpuMetrics {
		c.processGPUWeightedUtil(metrics, gpuUUID, gpuMetricList)
	}
}

// processGPUWeightedUtil processes weighted utilization for a single GPU
func (c *DCGMCollector) processGPUWeightedUtil(metrics MetricsByCounter, gpuUUID string, gpuMetricList []Metric) {
	// Check if this GPU is in MIG mode
	migMode := c.getMIGMode(gpuMetricList)

	if migMode == "1" {
		// MIG mode: calculate weighted utilization
		c.calculateMIGWeightedUtil(metrics, gpuUUID, gpuMetricList)
	} else if migMode == "0" {
		// Non-MIG mode: use GPU_UTIL directly
		c.calculateNonMIGWeightedUtil(metrics, gpuUUID, gpuMetricList)
	} else {
		slog.Debug("Unknown MIG mode, skipping weighted util", "gpu", gpuUUID, "mig_mode", migMode)
	}
}

// getMIGMode extracts MIG mode from GPU metrics
func (c *DCGMCollector) getMIGMode(gpuMetricList []Metric) string {
	for _, metric := range gpuMetricList {
		if migMode, exists := metric.Labels["DCGM_FI_DEV_MIG_MODE"]; exists {
			return migMode
		}
	}
	return "0" // Default to non-MIG mode
}

// calculateMIGWeightedUtil calculates weighted utilization for MIG GPU
func (c *DCGMCollector) calculateMIGWeightedUtil(metrics MetricsByCounter, gpuUUID string, gpuMetricList []Metric) {
	// Find all MIG instances for this GPU
	migInstances := make(map[string]Metric) // GPU_I_ID -> Metric
	var maxSlices int
	var sampleMetric Metric

	for _, metric := range gpuMetricList {
		// Look for DCGM_FI_PROF_GR_ENGINE_ACTIVE metrics
		if metric.Counter.FieldName == "DCGM_FI_PROF_GR_ENGINE_ACTIVE" && metric.GPUInstanceID != "" {
			migInstances[metric.GPUInstanceID] = metric
			sampleMetric = metric
		}

		// Extract max slices from any metric with this label
		if maxSlicesStr, exists := metric.Labels["DCGM_FI_DEV_MIG_MAX_SLICES"]; exists && maxSlices == 0 {
			if ms, err := strconv.Atoi(maxSlicesStr); err == nil {
				maxSlices = ms
			}
		}
	}

	if len(migInstances) == 0 {
		return // Cannot calculate without instance activity
	}

	if maxSlices == 0 {
		// Fallback default commonly 7 for A100; log for visibility
		slog.Debug("DCGM_FI_DEV_MIG_MAX_SLICES not found, using default", "gpu", gpuUUID, "default", 7)
		maxSlices = 7
	}

	// Calculate weighted sum
	var weightedSum float64
	for _, migMetric := range migInstances {
		// Extract compute slices from MIG profile
		computeSlices := c.extractComputeSlices(migMetric.MigProfile)
		if computeSlices == 0 {
			continue
		}

		// Parse engine active value (already 0..1 ratio)
		engineActive, err := strconv.ParseFloat(migMetric.Value, 64)
		if err != nil {
			continue
		}

		// Calculate weighted contribution
		sliceRatio := float64(computeSlices) / float64(maxSlices)
		weightedSum += engineActive * sliceRatio
	}

	// Create weighted GPU utilization metric
	c.createWeightedGPUUtilMetric(metrics, sampleMetric, weightedSum, "weighted_sum")
}

// calculateNonMIGWeightedUtil calculates weighted utilization for non-MIG GPU
func (c *DCGMCollector) calculateNonMIGWeightedUtil(metrics MetricsByCounter, gpuUUID string, gpuMetricList []Metric) {
	// Find GPU_UTIL metric
	for _, metric := range gpuMetricList {
		if metric.Counter.FieldName == "DCGM_FI_DEV_GPU_UTIL" {
			// Convert percentage to ratio (0-100 -> 0-1)
			gpuUtil, err := strconv.ParseFloat(metric.Value, 64)
			if err != nil {
				continue
			}

			weightedUtil := gpuUtil / 100.0
			c.createWeightedGPUUtilMetric(metrics, metric, weightedUtil, "direct")
			break
		}
	}
}

// extractComputeSlices extracts compute slices from MIG profile name
func (c *DCGMCollector) extractComputeSlices(migProfile string) int {
	// Pattern to match MIG profiles like "1g.5gb", "2g.10gb", etc.
	re := regexp.MustCompile(`^(\d+)g\.`)
	matches := re.FindStringSubmatch(migProfile)

	if len(matches) >= 2 {
		if slices, err := strconv.Atoi(matches[1]); err == nil {
			return slices
		}
	}

	return 0
}

// createWeightedGPUUtilMetric creates a new weighted GPU utilization metric
func (c *DCGMCollector) createWeightedGPUUtilMetric(metrics MetricsByCounter, sampleMetric Metric, value float64, calculationMethod string) {
	// Create counter for weighted GPU util
	weightedCounter := counters.Counter{
		FieldID:   dcgm.Short(counters.DCGMWeightedGPUUtil),
		FieldName: counters.DCGMExpWeightedGPUUtil,
		PromType:  "gauge",
		Help:      "Weighted GPU utilization for MIG and non-MIG devices",
	}

	// Create labels (copy from sample metric and add calculation method)
	labels := make(map[string]string)
	for k, v := range sampleMetric.Labels {
		labels[k] = v
	}
	labels["calculation_method"] = calculationMethod

	// Create the metric
	weightedMetric := Metric{
		Counter: weightedCounter,
		Value:   fmt.Sprintf("%.6f", value),

		UUID:         sampleMetric.UUID,
		GPU:          sampleMetric.GPU,
		GPUUUID:      sampleMetric.GPUUUID,
		GPUDevice:    sampleMetric.GPUDevice,
		GPUModelName: sampleMetric.GPUModelName,
		GPUPCIBusID:  sampleMetric.GPUPCIBusID,
		Hostname:     sampleMetric.Hostname,

		Labels:        labels,
		Attributes:    nil,
		MigProfile:    "", // Clear MIG profile for aggregated metric
		GPUInstanceID: "", // Clear instance ID for aggregated metric
	}

	// Add to metrics
	metrics[weightedCounter] = append(metrics[weightedCounter], weightedMetric)
}
