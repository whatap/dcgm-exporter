package collector

import (
	"testing"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	mockdcgm "github.com/NVIDIA/dcgm-exporter/internal/mocks/pkg/dcgmprovider"
	mockdevicewatcher "github.com/NVIDIA/dcgm-exporter/internal/mocks/pkg/devicewatcher"
	"github.com/NVIDIA/dcgm-exporter/internal/pkg/dcgmprovider"
)

const testProfilingResetAfter = 10 * time.Second

// staleProfilingHarness drives a one-GPU profiling collector whose re-watch always
// succeeds while the profiling timestamp only advances when the test changes it.
type staleProfilingHarness struct {
	t         *testing.T
	mockDCGM  *mockdcgm.MockDCGM
	collector *DCGMCollector
	start     time.Time
	now       time.Time
	timestamp int64
	calls     []string
	reads     int
	// onRead runs after reads is incremented and before the sample is returned.
	onRead func()
}

func newStaleProfilingHarness(t *testing.T, resetAfter time.Duration) *staleProfilingHarness {
	t.Helper()
	realDCGM := dcgmprovider.Client()
	t.Cleanup(func() { dcgmprovider.SetClient(realDCGM) })

	ctrl := gomock.NewController(t)
	mockDCGM := mockdcgm.NewMockDCGM(ctrl)
	dcgmprovider.SetClient(mockDCGM)
	deviceInfo := testGPUDeviceInfo(ctrl)
	watcher := mockdevicewatcher.NewMockWatcher(ctrl)
	watcher.EXPECT().WatchDeviceFieldGroups(gomock.Any(), deviceInfo).Return(nil, nil, nil, nil).AnyTimes()
	mockDCGM.EXPECT().UpdateAllFields().Return(nil).AnyTimes()

	h := &staleProfilingHarness{
		t:         t,
		mockDCGM:  mockDCGM,
		collector: newTestProfilingCollector(t, deviceInfo, watcher, time.Second),
		start:     time.Unix(100, 0),
		timestamp: 10,
	}
	h.now = h.start
	h.collector.now = func() time.Time { return h.now }
	h.collector.profilingResetAfter = resetAfter
	h.collector.profilingResetter = &profilingResetter{}

	fieldID := testProfilingCounter().FieldID
	mockDCGM.EXPECT().
		EntityGetLatestValues(dcgm.FE_GPU, uint(0), []dcgm.Short{fieldID}).
		DoAndReturn(func(dcgm.Field_Entity_Group, uint, []dcgm.Short) ([]dcgm.FieldValue_v1, error) {
			h.reads++
			if h.onRead != nil {
				h.onRead()
			}
			return []dcgm.FieldValue_v1{profilingFieldValue(fieldID, dcgm.DCGM_ST_OK, h.timestamp, 0)}, nil
		}).
		AnyTimes()

	// The first collection only establishes the freshness baseline.
	h.scrapeAt(0)
	return h
}

// expectResets records pause/resume calls; gomock rejects any call the test did not allow.
func (h *staleProfilingHarness) expectResets(times int) {
	h.mockDCGM.EXPECT().ProfPause().DoAndReturn(func() error {
		h.calls = append(h.calls, "pause")
		return nil
	}).Times(times)
	h.mockDCGM.EXPECT().ProfResume().DoAndReturn(func() error {
		h.calls = append(h.calls, "resume")
		return nil
	}).Times(times)
}

// scrapeAt moves the fake clock to start+offset and runs one collection.
func (h *staleProfilingHarness) scrapeAt(offset time.Duration) MetricsByCounter {
	h.t.Helper()
	h.now = h.start.Add(offset)
	got, err := h.collector.GetMetrics()
	require.NoError(h.t, err)
	return got
}

// TestDCGMCollectorResetsProfilingAfterPersistentStaleRepair verifies the escalation
// threshold and that a repeated reset waits for the reset interval.
func TestDCGMCollectorResetsProfilingAfterPersistentStaleRepair(t *testing.T) {
	h := newStaleProfilingHarness(t, testProfilingResetAfter)
	h.expectResets(2)

	// The first failed repair starts the streak without touching DCGM profiling.
	assert.Empty(t, h.scrapeAt(3*time.Second))
	assert.Empty(t, h.calls)

	// Re-watching keeps failing for the full reset window: pause, then resume.
	assert.Empty(t, h.scrapeAt(13*time.Second))
	assert.Equal(t, []string{"pause", "resume"}, h.calls)

	// A still-stale stream cannot reset again inside the reset interval.
	h.scrapeAt(16 * time.Second)
	assert.Len(t, h.calls, 2)

	// After the interval the reset is retried for the same streak.
	h.scrapeAt(24 * time.Second)
	assert.Equal(t, []string{"pause", "resume", "pause", "resume"}, h.calls)
}

// TestDCGMCollectorDoesNotResetProfilingWhenDisabled keeps the upstream repair-only behavior.
func TestDCGMCollectorDoesNotResetProfilingWhenDisabled(t *testing.T) {
	h := newStaleProfilingHarness(t, 0)

	// gomock fails the test on any ProfPause/ProfResume call.
	for _, offset := range []time.Duration{3 * time.Second, 13 * time.Second, time.Minute} {
		assert.Empty(t, h.scrapeAt(offset))
	}
}

// TestDCGMCollectorHealthyScrapeRestartsProfilingResetStreak verifies that a recovered
// stream must fail for a full window again before profiling is reset.
func TestDCGMCollectorHealthyScrapeRestartsProfilingResetStreak(t *testing.T) {
	h := newStaleProfilingHarness(t, testProfilingResetAfter)
	h.expectResets(1)

	h.scrapeAt(3 * time.Second) // streak starts at 3s

	h.timestamp = 11
	require.NotEmpty(t, h.scrapeAt(5*time.Second)) // fresh sample clears the streak

	h.scrapeAt(8 * time.Second)  // new streak starts at 8s
	h.scrapeAt(14 * time.Second) // 11s after the first streak, only 6s into the new one
	assert.Empty(t, h.calls)

	h.scrapeAt(18 * time.Second)
	assert.Equal(t, []string{"pause", "resume"}, h.calls)
}

// TestDCGMCollectorSuccessfulRepairClearsProfilingResetStreak verifies that a re-watch that
// recovers fresh samples does not leave a streak behind.
func TestDCGMCollectorSuccessfulRepairClearsProfilingResetStreak(t *testing.T) {
	h := newStaleProfilingHarness(t, testProfilingResetAfter)

	h.scrapeAt(3 * time.Second)
	require.False(t, h.collector.profilingRepairFailingSince.IsZero())

	// The next collection still reads the frozen sample, but the post-repair retry is fresh.
	staleRead := h.reads + 1
	h.onRead = func() {
		if h.reads == staleRead+1 {
			h.timestamp = 12
		}
	}

	require.NotEmpty(t, h.scrapeAt(6*time.Second))
	assert.Equal(t, staleRead+1, h.reads)
	assert.True(t, h.collector.profilingRepairFailingSince.IsZero())
}

func testProfilingRepairReason() *profilingRepairReason {
	return &profilingRepairReason{
		key: profilingSampleKey{
			entityGroup: dcgm.FE_GPU_I,
			fieldID:     dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE,
		},
		timestamp: 10,
		staleFor:  time.Minute,
	}
}

func newMockProfilingClient(t *testing.T) *mockdcgm.MockDCGM {
	t.Helper()
	realDCGM := dcgmprovider.Client()
	t.Cleanup(func() { dcgmprovider.SetClient(realDCGM) })
	mockDCGM := mockdcgm.NewMockDCGM(gomock.NewController(t))
	dcgmprovider.SetClient(mockDCGM)
	return mockDCGM
}

// TestProfilingResetterRetriesFailedResume verifies that a failed resume is retried before
// any rate limit so DCGM profiling is never left paused by the exporter.
func TestProfilingResetterRetriesFailedResume(t *testing.T) {
	mockDCGM := newMockProfilingClient(t)
	resetter := &profilingResetter{}
	now := time.Unix(100, 0)

	gomock.InOrder(
		mockDCGM.EXPECT().ProfPause().Return(nil),
		mockDCGM.EXPECT().ProfResume().Return(assert.AnError),
		mockDCGM.EXPECT().ProfResume().Return(nil),
	)

	assert.False(t, resetter.resetIfDue(now, testProfilingResetAfter, testProfilingResetAfter, testProfilingRepairReason()))
	assert.True(t, resetter.resetIfDue(now.Add(time.Second), 0, testProfilingResetAfter, testProfilingRepairReason()))
	assert.False(t, resetter.resetIfDue(now.Add(2*time.Second), time.Hour, testProfilingResetAfter, testProfilingRepairReason()))
}

// TestProfilingResetterSkipsResumeWhenPauseFails verifies that a failed pause is rate limited
// and never followed by an unmatched resume.
func TestProfilingResetterSkipsResumeWhenPauseFails(t *testing.T) {
	mockDCGM := newMockProfilingClient(t)
	resetter := &profilingResetter{}
	now := time.Unix(100, 0)

	mockDCGM.EXPECT().ProfPause().Return(assert.AnError)

	assert.False(t, resetter.resetIfDue(now, testProfilingResetAfter, testProfilingResetAfter, testProfilingRepairReason()))
	assert.False(t, resetter.resetIfDue(now.Add(time.Second), time.Hour, testProfilingResetAfter, testProfilingRepairReason()))
}

// TestProfilingResetterIsSharedAcrossCollectors verifies one hostengine-wide reset when
// several entity collectors escalate in the same window.
func TestProfilingResetterIsSharedAcrossCollectors(t *testing.T) {
	mockDCGM := newMockProfilingClient(t)
	resetter := &profilingResetter{}
	now := time.Unix(100, 0)

	mockDCGM.EXPECT().ProfPause().Return(nil)
	mockDCGM.EXPECT().ProfResume().Return(nil)

	gpuCollector := &DCGMCollector{profilingResetAfter: testProfilingResetAfter, profilingResetter: resetter}
	migCollector := &DCGMCollector{profilingResetAfter: testProfilingResetAfter, profilingResetter: resetter}
	for _, collector := range []*DCGMCollector{gpuCollector, migCollector} {
		collector.profilingRepairFailingSince = now.Add(-testProfilingResetAfter)
		collector.escalateFailedProfilingRepair(now, testProfilingRepairReason())
	}
}
