package main

/*
#include <stdlib.h>
#include <stdint.h>
*/
import "C"

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
	"unsafe"

	"github.com/SpherePrime/CLI/internal/desktopengine"
)

var engineRegistry = struct {
	sync.Mutex
	next      uint64
	engines   map[uint64]*desktopengine.Engine
	lastError string
}{engines: make(map[uint64]*desktopengine.Engine)}

func recordError(err error) {
	engineRegistry.Lock()
	engineRegistry.lastError = err.Error()
	engineRegistry.Unlock()
}

func findEngine(handle C.uint64_t) (*desktopengine.Engine, error) {
	engineRegistry.Lock()
	engine, ok := engineRegistry.engines[uint64(handle)]
	engineRegistry.Unlock()
	if !ok {
		return nil, errors.New("unknown engine handle")
	}
	return engine, nil
}

//export prime_engine_abi_version
func prime_engine_abi_version() C.uint32_t { return desktopengine.ABIVersion }

//export prime_engine_last_error
func prime_engine_last_error() *C.char {
	engineRegistry.Lock()
	defer engineRegistry.Unlock()
	return C.CString(engineRegistry.lastError)
}

//export prime_engine_create
func prime_engine_create(optionsJSON *C.char) C.uint64_t {
	var options desktopengine.Options
	if optionsJSON != nil {
		if err := json.Unmarshal([]byte(C.GoString(optionsJSON)), &options); err != nil {
			recordError(err)
			return 0
		}
	}
	engine, err := desktopengine.New(options)
	if err != nil {
		recordError(err)
		return 0
	}
	engineRegistry.Lock()
	engineRegistry.next++
	handle := engineRegistry.next
	engineRegistry.engines[handle] = engine
	engineRegistry.Unlock()
	return C.uint64_t(handle)
}

//export prime_engine_request
func prime_engine_request(handle C.uint64_t, requestJSON *C.char) C.uint64_t {
	engine, err := findEngine(handle)
	if err != nil {
		recordError(err)
		return 0
	}
	if requestJSON == nil {
		recordError(errors.New("request is required"))
		return 0
	}
	var request desktopengine.Request
	if err := json.Unmarshal([]byte(C.GoString(requestJSON)), &request); err != nil {
		recordError(err)
		return 0
	}
	id, err := engine.Request(request)
	if err != nil {
		recordError(err)
		return 0
	}
	return C.uint64_t(id)
}

//export prime_engine_read
func prime_engine_read(handle C.uint64_t, request C.uint64_t, timeoutMS C.uint32_t) *C.char {
	engine, err := findEngine(handle)
	if err != nil {
		return errorEvent(err)
	}
	event, err := engine.Read(uint64(request), time.Duration(timeoutMS)*time.Millisecond)
	if err != nil {
		return errorEvent(err)
	}
	if event == nil {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return errorEvent(err)
	}
	return C.CString(string(data))
}

func errorEvent(err error) *C.char {
	data, _ := json.Marshal(desktopengine.Event{Type: "error", Error: err.Error()})
	return C.CString(string(data))
}

func requestAction(handle C.uint64_t, request C.uint64_t, release bool) C.int {
	engine, err := findEngine(handle)
	if err == nil {
		if release {
			err = engine.Release(uint64(request))
		} else {
			err = engine.Cancel(uint64(request))
		}
	}
	if err != nil {
		recordError(err)
		return -1
	}
	return 0
}

//export prime_engine_cancel
func prime_engine_cancel(handle C.uint64_t, request C.uint64_t) C.int {
	return requestAction(handle, request, false)
}

//export prime_engine_release_request
func prime_engine_release_request(handle C.uint64_t, request C.uint64_t) C.int {
	return requestAction(handle, request, true)
}

//export prime_engine_free
func prime_engine_free(buffer *C.char) { C.free(unsafe.Pointer(buffer)) }

//export prime_engine_shutdown
func prime_engine_shutdown(handle C.uint64_t) C.int {
	engineRegistry.Lock()
	engine, ok := engineRegistry.engines[uint64(handle)]
	if ok {
		delete(engineRegistry.engines, uint64(handle))
	}
	engineRegistry.Unlock()
	if !ok {
		recordError(errors.New("unknown engine handle"))
		return -1
	}
	engine.Close()
	return 0
}

func main() {}
