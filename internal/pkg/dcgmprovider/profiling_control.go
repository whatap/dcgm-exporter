package dcgmprovider

/*
#cgo linux LDFLAGS: -ldl
#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif
#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>

// Mirrors dcgmReturn_t dcgmProfPause/dcgmProfResume(dcgmHandle_t) from dcgm_agent.h,
// where dcgmReturn_t is an int-sized enum and dcgmHandle_t is uintptr_t.
typedef int (*exporter_dcgm_prof_control_fn)(uintptr_t);

// exporter_dcgm_prof_control resolves the symbol from the libdcgm that go-dcgm loaded
// with RTLD_GLOBAL, so a missing library or symbol is reported instead of crashing.
static int exporter_dcgm_prof_control(const char *symbol, uintptr_t handle, int *found) {
	exporter_dcgm_prof_control_fn fn = (exporter_dcgm_prof_control_fn)dlsym(RTLD_DEFAULT, symbol);
	if (fn == NULL) {
		*found = 0;
		return 0;
	}
	*found = 1;
	return fn(handle);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// goDCGMHandle aliases the process-wide handle that go-dcgm's dcgm.Init established for
// either embedded or standalone mode. go-dcgm v1.4601 does not wrap dcgmProfPause or
// dcgmProfResume, and both calls must use that same connection. The layout mirrors
// go-dcgm's `type dcgmHandle struct{ handle C.dcgmHandle_t }`; if go-dcgm renames the
// variable the binary fails to link instead of silently using another handle.
//
//go:linkname goDCGMHandle github.com/NVIDIA/go-dcgm/pkg/dcgm.handle
var goDCGMHandle struct{ handle uintptr }

const (
	dcgmProfPauseSymbol  = "dcgmProfPause"
	dcgmProfResumeSymbol = "dcgmProfResume"
)

// ProfPause pauses DCGM profiling (DCP) collection for every GPU on the hostengine.
func (d dcgmProvider) ProfPause() error {
	return callProfilingControl(dcgmProfPauseSymbol)
}

// ProfResume resumes DCGM profiling collection and re-acquires its per-GPU state.
func (d dcgmProvider) ProfResume() error {
	return callProfilingControl(dcgmProfResumeSymbol)
}

// callProfilingControl invokes one DCGM profiling-control API on go-dcgm's handle.
func callProfilingControl(symbol string) error {
	cSymbol := C.CString(symbol)
	defer C.free(unsafe.Pointer(cSymbol))

	var found C.int
	result := C.exporter_dcgm_prof_control(cSymbol, C.uintptr_t(goDCGMHandle.handle), &found)
	if found == 0 {
		return fmt.Errorf("%s is not available in the loaded DCGM library", symbol)
	}
	if result != 0 {
		return fmt.Errorf("%s failed with DCGM status %d", symbol, int(result))
	}
	return nil
}
