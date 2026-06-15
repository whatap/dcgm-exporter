package transformation

import (
	"strconv"
	"testing"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"

	"github.com/NVIDIA/dcgm-exporter/internal/pkg/collector"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/counters"
)

func TestIsHSeriesGPU(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"NVIDIA H100 80GB HBM3", true},
		{"NVIDIA H200", true},
		{"NVIDIA H800", true},
		{"NVIDIA H20", true},
		{"NVIDIA A100 80GB", false},
		{"NVIDIA V100", false},
		{"NVIDIA L40", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHSeriesGPU(tt.model); got != tt.expected {
			t.Errorf("isHSeriesGPU(%q) = %v, want %v", tt.model, got, tt.expected)
		}
	}
}

func TestComputeNonMIG_CapsAt100(t *testing.T) {
	w := NewWeightedUtil()

	utilCounter := counters.Counter{
		FieldID:   dcgm.Short(gpuUtilID),
		FieldName: "DCGM_FI_DEV_GPU_UTIL",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		utilCounter: {
			{
				GPU:          "0",
				GPUUUID:      "GPU-abc",
				GPUModelName: "NVIDIA A100",
				Value:        "250",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
			{
				GPU:          "1",
				GPUUUID:      "GPU-def",
				GPUModelName: "NVIDIA A100",
				Value:        "50",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
	}

	hSeriesGPUs := make(map[string]bool)
	result := w.computeNonMIG(metrics, hSeriesGPUs)

	if len(result) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(result))
	}

	// GPU 0: value was 250, should be capped at 100, so weighted = 1.0
	val0, _ := strconv.ParseFloat(result[0].Value, 64)
	if result[0].GPU == "0" {
		if val0 != 1.0 {
			t.Errorf("GPU 0: expected weighted value 1.0, got %f", val0)
		}
	}

	// GPU 1: value was 50, weighted = 0.5
	for _, m := range result {
		if m.GPU == "1" {
			val, _ := strconv.ParseFloat(m.Value, 64)
			if val != 0.5 {
				t.Errorf("GPU 1: expected weighted value 0.5, got %f", val)
			}
		}
	}
}

func TestComputeNonMIG_SkipsHSeriesGPUs(t *testing.T) {
	w := NewWeightedUtil()

	utilCounter := counters.Counter{
		FieldID:   dcgm.Short(gpuUtilID),
		FieldName: "DCGM_FI_DEV_GPU_UTIL",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		utilCounter: {
			{
				GPU:          "0",
				GPUUUID:      "GPU-h100",
				GPUModelName: "NVIDIA H100 80GB HBM3",
				Value:        "80",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
			{
				GPU:          "1",
				GPUUUID:      "GPU-a100",
				GPUModelName: "NVIDIA A100",
				Value:        "50",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
	}

	hSeriesGPUs := map[string]bool{"0": true}
	result := w.computeNonMIG(metrics, hSeriesGPUs)

	if len(result) != 1 {
		t.Fatalf("expected 1 metric (only non-H-series), got %d", len(result))
	}
	if result[0].GPU != "1" {
		t.Errorf("expected GPU 1 (A100), got GPU %s", result[0].GPU)
	}
}

func TestComputeHSeriesNonMIG(t *testing.T) {
	w := NewWeightedUtil()

	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		activeCounter: {
			// Non-MIG H-series metric
			{
				GPU:          "0",
				GPUUUID:      "GPU-h100",
				GPUModelName: "NVIDIA H100 80GB HBM3",
				MigProfile:   "",
				Value:        "0.75",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
			// MIG metric (should be skipped)
			{
				GPU:          "0",
				GPUUUID:      "GPU-h100-mig",
				GPUModelName: "NVIDIA H100 80GB HBM3",
				MigProfile:   "1g.10gb",
				Value:        "0.5",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
			// Non-H-series metric (should be skipped)
			{
				GPU:          "1",
				GPUUUID:      "GPU-a100",
				GPUModelName: "NVIDIA A100",
				MigProfile:   "",
				Value:        "0.6",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
	}

	hSeriesGPUs := map[string]bool{"0": true}
	result := w.computeHSeriesNonMIG(metrics, hSeriesGPUs)

	if len(result) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(result))
	}

	if result[0].GPU != "0" {
		t.Errorf("expected GPU 0, got GPU %s", result[0].GPU)
	}

	val, _ := strconv.ParseFloat(result[0].Value, 64)
	if val != 0.75 {
		t.Errorf("expected value 0.75, got %f", val)
	}

	if result[0].Labels["calculation_method"] != "prof_gr_engine_active" {
		t.Errorf("expected calculation_method=prof_gr_engine_active, got %s", result[0].Labels["calculation_method"])
	}
}

func TestFindHSeriesGPUs(t *testing.T) {
	w := NewWeightedUtil()

	utilCounter := counters.Counter{
		FieldID:   dcgm.Short(gpuUtilID),
		FieldName: "DCGM_FI_DEV_GPU_UTIL",
		PromType:  "gauge",
	}
	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		utilCounter: {
			{GPU: "0", GPUModelName: "NVIDIA H100 80GB HBM3", Value: "50", Labels: map[string]string{}, Attributes: map[string]string{}},
			{GPU: "1", GPUModelName: "NVIDIA A100", Value: "50", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
		activeCounter: {
			{GPU: "0", MigProfile: "", Value: "0.5", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
	}

	result := w.findHSeriesGPUs(metrics)

	if !result["0"] {
		t.Error("expected GPU 0 (H100) to be identified as H-series")
	}
	if result["1"] {
		t.Error("expected GPU 1 (A100) to NOT be identified as H-series")
	}
}

func TestProcess_HSeriesUsesActiveInsteadOfUtil(t *testing.T) {
	w := NewWeightedUtil()

	utilCounter := counters.Counter{
		FieldID:   dcgm.Short(gpuUtilID),
		FieldName: "DCGM_FI_DEV_GPU_UTIL",
		PromType:  "gauge",
	}
	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		utilCounter: {
			{
				GPU:          "0",
				GPUUUID:      "GPU-h100",
				GPUModelName: "NVIDIA H100 80GB HBM3",
				Value:        "250", // abnormally high
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
		activeCounter: {
			{
				GPU:          "0",
				GPUUUID:      "GPU-h100",
				GPUModelName: "NVIDIA H100 80GB HBM3",
				MigProfile:   "",
				Value:        "0.85",
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
	}

	err := w.Process(metrics, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the weighted util metrics
	var weightedMetrics []collector.Metric
	for c, m := range metrics {
		if c.FieldName == counters.DCGMExpWeightedGPUUtil {
			weightedMetrics = m
			break
		}
	}

	if len(weightedMetrics) != 1 {
		t.Fatalf("expected 1 weighted metric, got %d", len(weightedMetrics))
	}

	// Should use PROF_GR_ENGINE_ACTIVE value (0.85), not GPU_UTIL (250)
	val, _ := strconv.ParseFloat(weightedMetrics[0].Value, 64)
	if val != 0.85 {
		t.Errorf("expected weighted value 0.85 (from PROF_GR_ENGINE_ACTIVE), got %f", val)
	}

	if weightedMetrics[0].Labels["calculation_method"] != "prof_gr_engine_active" {
		t.Errorf("expected calculation_method=prof_gr_engine_active, got %s", weightedMetrics[0].Labels["calculation_method"])
	}
}

// migTestMetrics builds a MetricsByCounter containing DCGM_FI_DEV_MIG_MAX_SLICES
// and DCGM_FI_PROF_GR_ENGINE_ACTIVE entries for a single physical GPU.
func migTestMetrics(gpu, uuid, model string, maxSlices string, instances []struct {
	profile string
	value   string
}) collector.MetricsByCounter {
	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}
	maxSlicesCounter := counters.Counter{
		FieldID:   dcgm.Short(migMaxSlicesID),
		FieldName: "DCGM_FI_DEV_MIG_MAX_SLICES",
		PromType:  "gauge",
	}

	activeMetrics := make([]collector.Metric, 0, len(instances))
	for i, inst := range instances {
		activeMetrics = append(activeMetrics, collector.Metric{
			GPU:           gpu,
			GPUUUID:       uuid,
			GPUModelName:  model,
			MigProfile:    inst.profile,
			GPUInstanceID: strconv.Itoa(i),
			Value:         inst.value,
			Labels:        map[string]string{},
			Attributes:    map[string]string{},
		})
	}

	return collector.MetricsByCounter{
		activeCounter: activeMetrics,
		maxSlicesCounter: {
			{
				GPU:          gpu,
				GPUUUID:      uuid,
				GPUModelName: model,
				Value:        maxSlices,
				Labels:       map[string]string{},
				Attributes:   map[string]string{},
			},
		},
	}
}

func approxEqual(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// TestComputeMIG_A100_PRDExample validates the exact worked example from the PRD:
// A100 (MAX_SLICES=7) with six 1g.5gb instances should sum to ~0.322663.
func TestComputeMIG_A100_PRDExample(t *testing.T) {
	w := NewWeightedUtil()

	instances := []struct {
		profile string
		value   string
	}{
		{"1g.5gb", "0.982262"},
		{"1g.5gb", "0.000002"},
		{"1g.5gb", "0.510287"},
		{"1g.5gb", "0.000003"},
		{"1g.5gb", "0.766027"},
		{"1g.5gb", "0.000069"},
	}

	metrics := migTestMetrics("1", "GPU-9dadccd1-6248-ac2a-6e85-0af3fdfeef3c", "NVIDIA A100-SXM4-40GB", "7", instances)

	result := w.computeMIG(metrics)
	if len(result) != 1 {
		t.Fatalf("expected 1 aggregated metric for the physical GPU, got %d", len(result))
	}

	val, err := strconv.ParseFloat(result[0].Value, 64)
	if err != nil {
		t.Fatalf("unparseable value %q: %v", result[0].Value, err)
	}
	if !approxEqual(val, 0.322663, 1e-5) {
		t.Errorf("expected weighted sum ~0.322663, got %.6f", val)
	}
	if result[0].Labels["calculation_method"] != "weighted_sum" {
		t.Errorf("expected calculation_method=weighted_sum, got %s", result[0].Labels["calculation_method"])
	}
	if result[0].Labels["DCGM_FI_DEV_UUID"] != "GPU-9dadccd1-6248-ac2a-6e85-0af3fdfeef3c" {
		t.Errorf("expected DCGM_FI_DEV_UUID label = GPUUUID, got %q", result[0].Labels["DCGM_FI_DEV_UUID"])
	}
}

// TestComputeMIG_VariousProfiles validates mixed slice profiles on one GPU.
func TestComputeMIG_VariousProfiles(t *testing.T) {
	w := NewWeightedUtil()

	// 2g.10gb @ 0.5 -> 0.5*(2/7); 3g.20gb @ 1.0 -> 1.0*(3/7)
	instances := []struct {
		profile string
		value   string
	}{
		{"2g.10gb", "0.5"},
		{"3g.20gb", "1.0"},
	}
	metrics := migTestMetrics("0", "GPU-mixed", "NVIDIA A100-SXM4-40GB", "7", instances)

	result := w.computeMIG(metrics)
	if len(result) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(result))
	}
	val, _ := strconv.ParseFloat(result[0].Value, 64)
	expected := 0.5*(2.0/7.0) + 1.0*(3.0/7.0)
	if !approxEqual(val, expected, 1e-9) {
		t.Errorf("expected %.9f, got %.9f", expected, val)
	}
}

// TestComputeMIG_Boundaries checks 0% and full (7g.40gb @ 1.0) utilization.
func TestComputeMIG_Boundaries(t *testing.T) {
	w := NewWeightedUtil()

	t.Run("all zero", func(t *testing.T) {
		instances := []struct {
			profile string
			value   string
		}{
			{"1g.5gb", "0.0"},
			{"1g.5gb", "0.0"},
		}
		metrics := migTestMetrics("0", "GPU-zero", "NVIDIA A100-SXM4-40GB", "7", instances)
		result := w.computeMIG(metrics)
		if len(result) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(result))
		}
		val, _ := strconv.ParseFloat(result[0].Value, 64)
		if !approxEqual(val, 0.0, 1e-12) {
			t.Errorf("expected 0.0, got %.9f", val)
		}
	})

	t.Run("full single slice profile", func(t *testing.T) {
		// A single 7g.40gb instance fully active occupies the whole GPU -> 1.0
		instances := []struct {
			profile string
			value   string
		}{
			{"7g.40gb", "1.0"},
		}
		metrics := migTestMetrics("0", "GPU-full", "NVIDIA A100-SXM4-40GB", "7", instances)
		result := w.computeMIG(metrics)
		if len(result) != 1 {
			t.Fatalf("expected 1 metric, got %d", len(result))
		}
		val, _ := strconv.ParseFloat(result[0].Value, 64)
		if !approxEqual(val, 1.0, 1e-12) {
			t.Errorf("expected 1.0, got %.9f", val)
		}
	})
}

// TestComputeMIG_MissingMaxSlicesFallback ensures the fallback default (7) is used
// when DCGM_FI_DEV_MIG_MAX_SLICES is absent for a GPU.
func TestComputeMIG_MissingMaxSlicesFallback(t *testing.T) {
	w := NewWeightedUtil()

	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}
	// No MAX_SLICES counter at all.
	metrics := collector.MetricsByCounter{
		activeCounter: {
			{GPU: "0", GPUUUID: "GPU-nomax", GPUModelName: "NVIDIA A100", MigProfile: "1g.5gb", Value: "0.7", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
	}

	result := w.computeMIG(metrics)
	if len(result) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(result))
	}
	val, _ := strconv.ParseFloat(result[0].Value, 64)
	// fallback maxSlices=7 -> 0.7*(1/7)
	if !approxEqual(val, 0.7*(1.0/7.0), 1e-9) {
		t.Errorf("expected %.9f, got %.9f", 0.7*(1.0/7.0), val)
	}
	// MIG-specific fields should be cleared in the best-effort metric.
	if result[0].MigProfile != "" {
		t.Errorf("expected MigProfile cleared, got %q", result[0].MigProfile)
	}
}

func TestGetSlicesFromProfile(t *testing.T) {
	w := NewWeightedUtil()
	tests := []struct {
		profile  string
		expected float64
	}{
		{"1g.5gb", 1.0},
		{"2g.10gb", 2.0},
		{"3g.20gb", 3.0},
		{"4g.20gb", 4.0},
		{"7g.40gb", 7.0},
		{"1g.10gb", 1.0}, // H100 profile
		{"5g.30gb", 5.0}, // generic parsing path
		{"", 0.0},
		{"garbage", 0.0},
	}
	for _, tt := range tests {
		if got := w.getSlicesFromProfile(tt.profile); got != tt.expected {
			t.Errorf("getSlicesFromProfile(%q) = %v, want %v", tt.profile, got, tt.expected)
		}
	}
}

// TestProcess_MixedMIGAndNonMIG validates a mixed environment: one MIG A100 GPU
// (weighted_sum) and one regular A100 GPU (direct) coexisting.
func TestProcess_MixedMIGAndNonMIG(t *testing.T) {
	w := NewWeightedUtil()

	utilCounter := counters.Counter{
		FieldID:   dcgm.Short(gpuUtilID),
		FieldName: "DCGM_FI_DEV_GPU_UTIL",
		PromType:  "gauge",
	}
	activeCounter := counters.Counter{
		FieldID:   dcgm.Short(profGrEngineActive),
		FieldName: "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
		PromType:  "gauge",
	}
	maxSlicesCounter := counters.Counter{
		FieldID:   dcgm.Short(migMaxSlicesID),
		FieldName: "DCGM_FI_DEV_MIG_MAX_SLICES",
		PromType:  "gauge",
	}

	metrics := collector.MetricsByCounter{
		// Regular A100 (GPU 1), not MIG
		utilCounter: {
			{GPU: "1", GPUUUID: "GPU-regular", GPUModelName: "NVIDIA A100-SXM4-40GB", Value: "60", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
		// MIG A100 (GPU 0): two 1g.5gb @ 0.7 -> 1.4/7 = 0.2
		activeCounter: {
			{GPU: "0", GPUUUID: "GPU-mig", GPUModelName: "NVIDIA A100-SXM4-40GB", MigProfile: "1g.5gb", GPUInstanceID: "0", Value: "0.7", Labels: map[string]string{}, Attributes: map[string]string{}},
			{GPU: "0", GPUUUID: "GPU-mig", GPUModelName: "NVIDIA A100-SXM4-40GB", MigProfile: "1g.5gb", GPUInstanceID: "1", Value: "0.7", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
		maxSlicesCounter: {
			{GPU: "0", GPUUUID: "GPU-mig", GPUModelName: "NVIDIA A100-SXM4-40GB", Value: "7", Labels: map[string]string{}, Attributes: map[string]string{}},
		},
	}

	if err := w.Process(metrics, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var weighted []collector.Metric
	for c, m := range metrics {
		if c.FieldName == counters.DCGMExpWeightedGPUUtil {
			weighted = m
			break
		}
	}
	if len(weighted) != 2 {
		t.Fatalf("expected 2 weighted metrics (1 direct + 1 weighted_sum), got %d", len(weighted))
	}

	byMethod := map[string]collector.Metric{}
	for _, m := range weighted {
		byMethod[m.Labels["calculation_method"]] = m
	}

	direct, ok := byMethod["direct"]
	if !ok {
		t.Fatal("missing direct (non-MIG) metric")
	}
	if v, _ := strconv.ParseFloat(direct.Value, 64); !approxEqual(v, 0.6, 1e-9) {
		t.Errorf("non-MIG: expected 0.6, got %s", direct.Value)
	}

	weightedSum, ok := byMethod["weighted_sum"]
	if !ok {
		t.Fatal("missing weighted_sum (MIG) metric")
	}
	if v, _ := strconv.ParseFloat(weightedSum.Value, 64); !approxEqual(v, 0.2, 1e-9) {
		t.Errorf("MIG: expected 0.2, got %s", weightedSum.Value)
	}
}
