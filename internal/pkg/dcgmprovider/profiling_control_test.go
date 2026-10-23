package dcgmprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProfilingControlWithoutLoadedDCGM verifies that profiling control reports an error
// instead of crashing when libdcgm has not been loaded by go-dcgm.
func TestProfilingControlWithoutLoadedDCGM(t *testing.T) {
	provider := dcgmProvider{}

	for name, call := range map[string]func() error{
		dcgmProfPauseSymbol:  provider.ProfPause,
		dcgmProfResumeSymbol: provider.ProfResume,
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)
			assert.Contains(t, err.Error(), name)
		})
	}
}

// TestProfilingControlUsesGoDCGMHandle verifies the linknamed handle starts at go-dcgm's zero value.
func TestProfilingControlUsesGoDCGMHandle(t *testing.T) {
	assert.Zero(t, goDCGMHandle.handle)
}
