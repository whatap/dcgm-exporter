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
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/appconfig"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/counters"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/dcgmprovider"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/deviceinfo"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/devicemonitoring"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/devicewatchlistmanager"
)

const unknownErr = "Unknown Error"

type DCGMCollector struct {
	counters                 []counters.Counter
	cleanups                 []func()
	useOldNamespace          bool
	deviceWatchList          devicewatchlistmanager.WatchList
	hostname                 string
	replaceBlanksInModelName bool
}

func NewDCGMCollector(
	c []counters.Counter,
	hostname string,
	config *appconfig.Config,
	deviceWatchList devicewatchlistmanager.WatchList,
) (*DCGMCollector, error) {
	if deviceWatchList.IsEmpty() {
		return nil, errors.New("deviceWatchList is empty")
	}

	collector := &DCGMCollector{
		counters:        c,
		deviceWatchList: deviceWatchList,
		hostname:        hostname,
	}

	if config == nil {
		slog.Warn("Config is empty")
		return collector, nil
	}

	collector.useOldNamespace = config.UseOldNamespace
	collector.replaceBlanksInModelName = config.ReplaceBlanksInModelName

	cleanups, err := deviceWatchList.Watch()
	if err != nil {
		return nil, err
	}

	collector.cleanups = cleanups

	return collector, nil
}

func (c *DCGMCollector) Cleanup() {
	for _, c := range c.cleanups {
		c()
	}
}

func (c *DCGMCollector) GetMetrics() (MetricsByCounter, error) {
	monitoringInfo := devicemonitoring.GetMonitoredEntities(c.deviceWatchList.DeviceInfo())

	metrics := make(MetricsByCounter)

	for _, mi := range monitoringInfo {
		var vals []dcgm.FieldValue_v1
		var err error
		if mi.Entity.EntityGroupId == dcgm.FE_LINK {
			vals, err = dcgmprovider.Client().LinkGetLatestValues(mi.Entity.EntityId, mi.ParentId,
				c.deviceWatchList.DeviceFields())
		} else {
			vals, err = dcgmprovider.Client().EntityGetLatestValues(mi.Entity.EntityGroupId, mi.Entity.EntityId,
				c.deviceWatchList.DeviceFields())
		}

		if err != nil {
			if derr, ok := err.(*dcgm.Error); ok {
				if derr.Code == dcgm.DCGM_ST_CONNECTION_NOT_VALID {
					slog.Error("Could not retrieve metrics: " + err.Error())
					os.Exit(1)
				}
			}
			return nil, err
		}

		// InstanceInfo will be nil for GPUs
		switch c.deviceWatchList.DeviceInfo().InfoType() {
		case dcgm.FE_SWITCH, dcgm.FE_LINK:
			toSwitchMetric(metrics, vals, c.counters, mi, c.useOldNamespace, c.hostname)
		case dcgm.FE_CPU, dcgm.FE_CPU_CORE:
			toCPUMetric(metrics, vals, c.counters, mi, c.useOldNamespace, c.hostname)
		default:
			toMetric(metrics,
				vals,
				c.counters,
				mi.DeviceInfo,
				mi.InstanceInfo,
				c.useOldNamespace,
				c.hostname,
				c.replaceBlanksInModelName)
		}
	}

	// Calculate weighted GPU utilization for MIG and non-MIG devices
	c.calculateWeightedGPUUtil(metrics)

	return metrics, nil
}

func findCounterField(c []counters.Counter, fieldID dcgm.Short) (counters.Counter, error) {
	for i := 0; i < len(c); i++ {
		if c[i].FieldID == fieldID {
			return c[i], nil
		}
	}

	return counters.Counter{}, fmt.Errorf("could not find counter corresponding to field ID '%d'", fieldID)
}

func toSwitchMetric(
	metrics MetricsByCounter,
	values []dcgm.FieldValue_v1, c []counters.Counter, mi devicemonitoring.Info, useOld bool, hostname string,
) {
	labels := map[string]string{}

	for _, val := range values {
		v := toString(val)
		// Filter out counters with no value and ignored fields for this entity

		counter, err := findCounterField(c, val.FieldID)
		if err != nil {
			continue
		}

		if counter.IsLabel() {
			labels[counter.FieldName] = v
			continue
		}
		uuid := "UUID"
		if useOld {
			uuid = "uuid"
		}
		var m Metric
		if v == skipDCGMValue {
			continue
		} else {
			m = Metric{
				Counter:      counter,
				Value:        v,
				UUID:         uuid,
				GPU:          fmt.Sprintf("%d", mi.Entity.EntityId),
				GPUUUID:      "",
				GPUDevice:    fmt.Sprintf("nvswitch%d", mi.ParentId),
				GPUModelName: "",
				GPUPCIBusID:  "",
				Hostname:     hostname,
				Labels:       labels,
				Attributes:   nil,
			}
		}

		metrics[m.Counter] = append(metrics[m.Counter], m)
	}
}

func toCPUMetric(
	metrics MetricsByCounter,
	values []dcgm.FieldValue_v1, c []counters.Counter, mi devicemonitoring.Info, useOld bool, hostname string,
) {
	labels := map[string]string{}

	for _, val := range values {
		v := toString(val)
		// Filter out counters with no value and ignored fields for this entity

		counter, err := findCounterField(c, val.FieldID)
		if err != nil {
			continue
		}

		if counter.IsLabel() {
			labels[counter.FieldName] = v
			continue
		}
		uuid := "UUID"
		if useOld {
			uuid = "uuid"
		}
		var m Metric
		if v == skipDCGMValue {
			continue
		} else {
			m = Metric{
				Counter:      counter,
				Value:        v,
				UUID:         uuid,
				GPU:          fmt.Sprintf("%d", mi.Entity.EntityId),
				GPUUUID:      "",
				GPUDevice:    fmt.Sprintf("%d", mi.ParentId),
				GPUModelName: "",
				GPUPCIBusID:  "",
				Hostname:     hostname,
				Labels:       labels,
				Attributes:   nil,
			}
		}

		metrics[m.Counter] = append(metrics[m.Counter], m)
	}
}

func toMetric(
	metrics MetricsByCounter,
	values []dcgm.FieldValue_v1,
	c []counters.Counter,
	d dcgm.Device,
	instanceInfo *deviceinfo.GPUInstanceInfo,
	useOld bool,
	hostname string,
	replaceBlanksInModelName bool,
) {
	labels := map[string]string{}

	for _, val := range values {
		v := toString(val)
		// Filter out counters with no value and ignored fields for this entity
		if v == skipDCGMValue {
			continue
		}

		counter, err := findCounterField(c, val.FieldID)
		if err != nil {
			continue
		}

		if counter.IsLabel() {
			labels[counter.FieldName] = v
			continue
		}
		uuid := "UUID"
		if useOld {
			uuid = "uuid"
		}

		gpuModel := getGPUModel(d, replaceBlanksInModelName)

		attrs := map[string]string{}
		if counter.FieldID == dcgm.DCGM_FI_DEV_XID_ERRORS {
			errCode := int(val.Int64())
			attrs["err_code"] = strconv.Itoa(errCode)
			if 0 <= errCode && errCode < len(xidErrCodeToText) {
				attrs["err_msg"] = xidErrCodeToText[errCode]
			} else {
				attrs["err_msg"] = unknownErr
			}
		}

		m := Metric{
			Counter: counter,
			Value:   v,

			UUID:         uuid,
			GPU:          fmt.Sprintf("%d", d.GPU),
			GPUUUID:      d.UUID,
			GPUDevice:    fmt.Sprintf("nvidia%d", d.GPU),
			GPUModelName: gpuModel,
			GPUPCIBusID:  d.PCI.BusID,
			Hostname:     hostname,

			Labels:     labels,
			Attributes: attrs,
		}
		if instanceInfo != nil {
			m.MigProfile = instanceInfo.ProfileName
			m.GPUInstanceID = fmt.Sprintf("%d", instanceInfo.Info.NvmlInstanceId)
		} else {
			m.MigProfile = ""
			m.GPUInstanceID = ""
		}

		metrics[m.Counter] = append(metrics[m.Counter], m)
	}
}

func getGPUModel(d dcgm.Device, replaceBlanksInModelName bool) string {
	gpuModel := d.Identifiers.Model

	if replaceBlanksInModelName {
		parts := strings.Fields(gpuModel)
		gpuModel = strings.Join(parts, " ")
		gpuModel = strings.ReplaceAll(gpuModel, " ", "-")
	}
	return gpuModel
}

func toString(value dcgm.FieldValue_v1) string {
	switch value.FieldType {
	case dcgm.DCGM_FT_INT64:
		switch v := value.Int64(); v {
		case dcgm.DCGM_FT_INT32_BLANK:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT32_NOT_FOUND:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT32_NOT_SUPPORTED:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT32_NOT_PERMISSIONED:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT64_BLANK:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT64_NOT_FOUND:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT64_NOT_SUPPORTED:
			return skipDCGMValue
		case dcgm.DCGM_FT_INT64_NOT_PERMISSIONED:
			return skipDCGMValue
		default:
			return fmt.Sprintf("%d", value.Int64())
		}
	case dcgm.DCGM_FT_DOUBLE:
		switch v := value.Float64(); v {
		case dcgm.DCGM_FT_FP64_BLANK:
			return skipDCGMValue
		case dcgm.DCGM_FT_FP64_NOT_FOUND:
			return skipDCGMValue
		case dcgm.DCGM_FT_FP64_NOT_SUPPORTED:
			return skipDCGMValue
		case dcgm.DCGM_FT_FP64_NOT_PERMISSIONED:
			return skipDCGMValue
		default:
			return fmt.Sprintf("%f", value.Float64())
		}
	case dcgm.DCGM_FT_STRING:
		switch v := value.String(); v {
		case dcgm.DCGM_FT_STR_BLANK:
			return skipDCGMValue
		case dcgm.DCGM_FT_STR_NOT_FOUND:
			return skipDCGMValue
		case dcgm.DCGM_FT_STR_NOT_SUPPORTED:
			return skipDCGMValue
		case dcgm.DCGM_FT_STR_NOT_PERMISSIONED:
			return skipDCGMValue
		default:
			return v
		}
	}

	return FailedToConvert
}

// calculateWeightedGPUUtil calculates weighted GPU utilization for MIG and non-MIG devices
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

	if len(migInstances) == 0 || maxSlices == 0 {
		return // Cannot calculate without required data
	}

	// Calculate weighted sum
	var weightedSum float64
	for _, migMetric := range migInstances {
		// Extract compute slices from MIG profile
		computeSlices := c.extractComputeSlices(migMetric.MigProfile)
		if computeSlices == 0 {
			continue
		}

		// Parse engine active value
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
		FieldName: "DCGM_FI_DEV_WEIGHTED_GPU_UTIL",
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
