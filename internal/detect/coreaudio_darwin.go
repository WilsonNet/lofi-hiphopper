//go:build darwin && cgo

package detect

/*
#cgo LDFLAGS: -framework CoreAudio -framework CoreFoundation
#include <AudioToolbox/AudioToolbox.h>
#include <CoreAudio/CoreAudio.h>
#include <libproc.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	int32_t pid;
	uint32_t input;
	uint32_t output;
	char name[128];
	char bundle[256];
	char path[1024];
} lofi_process;

// lofi_process_list returns the number of process objects, or -1 with status
// set. The caller frees *out with free().
static int lofi_process_list(lofi_process **out, OSStatus *status) {
	*out = NULL;
	*status = noErr;

	AudioObjectPropertyAddress address = {
		kAudioHardwarePropertyProcessObjectList,
		kAudioObjectPropertyScopeGlobal,
		kAudioObjectPropertyElementMain
	};

	UInt32 size = 0;
	OSStatus result = AudioObjectGetPropertyDataSize(
		kAudioObjectSystemObject, &address, 0, NULL, &size);
	if (result != noErr) {
		*status = result;
		return -1;
	}
	if (size == 0) {
		return 0;
	}

	UInt32 count = size / sizeof(AudioObjectID);
	AudioObjectID *objects = calloc(count, sizeof(AudioObjectID));
	if (objects == NULL) {
		*status = -1;
		return -1;
	}
	result = AudioObjectGetPropertyData(
		kAudioObjectSystemObject, &address, 0, NULL, &size, objects);
	if (result != noErr) {
		free(objects);
		*status = result;
		return -1;
	}
	count = size / sizeof(AudioObjectID);

	lofi_process *processes = calloc(count, sizeof(lofi_process));
	if (processes == NULL) {
		free(objects);
		*status = -1;
		return -1;
	}

	UInt32 used = 0;
	for (UInt32 i = 0; i < count; i++) {
		AudioObjectID object = objects[i];
		AudioObjectPropertyAddress prop = {
			kAudioProcessPropertyPID,
			kAudioObjectPropertyScopeGlobal,
			kAudioObjectPropertyElementMain
		};
		pid_t pid = 0;
		UInt32 dataSize = sizeof(pid);
		if (AudioObjectGetPropertyData(object, &prop, 0, NULL, &dataSize, &pid) != noErr || pid <= 0) {
			continue;
		}

		lofi_process process;
		memset(&process, 0, sizeof(process));
		process.pid = pid;

		prop.mSelector = kAudioProcessPropertyIsRunningInput;
		dataSize = sizeof(uint32_t);
		AudioObjectGetPropertyData(object, &prop, 0, NULL, &dataSize, &process.input);

		prop.mSelector = kAudioProcessPropertyIsRunningOutput;
		dataSize = sizeof(uint32_t);
		AudioObjectGetPropertyData(object, &prop, 0, NULL, &dataSize, &process.output);

		proc_name(pid, process.name, sizeof(process.name));
		proc_pidpath(pid, process.path, sizeof(process.path));

		CFStringRef bundle = NULL;
		prop.mSelector = kAudioProcessPropertyBundleID;
		dataSize = sizeof(bundle);
		if (AudioObjectGetPropertyData(object, &prop, 0, NULL, &dataSize, &bundle) == noErr && bundle != NULL) {
			CFStringGetCString(bundle, process.bundle, sizeof(process.bundle), kCFStringEncodingUTF8);
			CFRelease(bundle);
		}

		processes[used++] = process;
	}

	free(objects);
	*out = processes;
	return (int)used;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CoreAudio is the macOS audio-process probe. It uses the per-process API
// introduced in macOS 14.4 to see which apps have active output or input.
type CoreAudio struct{}

// New returns a CoreAudio probe.
func New() *CoreAudio { return &CoreAudio{} }

// Processes returns every CoreAudio process object it can inspect.
func (c *CoreAudio) Processes() ([]Process, error) {
	var raw *C.lofi_process
	var status C.OSStatus
	count := C.lofi_process_list(&raw, &status)
	if count < 0 {
		if int(status) == -1 {
			return nil, fmt.Errorf("CoreAudio process list: out of memory")
		}
		return nil, fmt.Errorf("CoreAudio process list failed (OSStatus %d); macOS 14.4+ is required", int32(status))
	}
	defer C.free(unsafe.Pointer(raw))
	if count == 0 {
		return nil, nil
	}

	processes := make([]Process, 0, int(count))
	for _, item := range unsafe.Slice(raw, int(count)) {
		name := C.GoString((*C.char)(unsafe.Pointer(&item.name[0])))
		bundle := C.GoString((*C.char)(unsafe.Pointer(&item.bundle[0])))
		path := C.GoString((*C.char)(unsafe.Pointer(&item.path[0])))
		processes = append(processes, Process{
			PID:      int(item.pid),
			Name:     name,
			BundleID: bundle,
			Path:     path,
			Input:    item.input != 0,
			Output:   item.output != 0,
		})
	}
	return processes, nil
}
