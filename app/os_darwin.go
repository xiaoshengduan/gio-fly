// SPDX-License-Identifier: Unlicense OR MIT

package app

/*
#include <Foundation/Foundation.h>

__attribute__ ((visibility ("hidden"))) void gio_wakeupMainThread(void);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createDisplayLink(uintptr_t handle);
__attribute__ ((visibility ("hidden"))) void gio_releaseDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) int gio_startDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) int gio_stopDisplayLink(CFTypeRef dl);
__attribute__ ((visibility ("hidden"))) void gio_setDisplayLinkDisplay(CFTypeRef dl, uint64_t did);
__attribute__ ((visibility ("hidden"))) void gio_hideCursor();
__attribute__ ((visibility ("hidden"))) void gio_showCursor();
__attribute__ ((visibility ("hidden"))) void gio_setCursor(NSUInteger curID);

static bool isMainThread() {
	return [NSThread isMainThread];
}

static NSUInteger nsstringLength(CFTypeRef cstr) {
	NSString *str = (__bridge NSString *)cstr;
	return [str length];
}

static void nsstringGetCharacters(CFTypeRef cstr, unichar *chars, NSUInteger loc, NSUInteger length) {
	NSString *str = (__bridge NSString *)cstr;
	[str getCharacters:chars range:NSMakeRange(loc, length)];
}

static CFTypeRef newNSString(unichar *chars, NSUInteger length) {
	@autoreleasepool {
		NSString *s = [NSString string];
		if (length > 0) {
			s = [NSString stringWithCharacters:chars length:length];
		}
		return CFBridgingRetain(s);
	}
}
*/
import "C"
import (
	"errors"
	"runtime/cgo"
	"sync/atomic"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/xiaoshengduan/gio-fly/io/pointer"
)

// displayLink is the state for a display link (CVDisplayLinkRef on macOS,
// CADisplayLink on iOS). It runs a state-machine goroutine that keeps the
// display link running for a while after being stopped to avoid the thread
// start/stop overhead and because the CVDisplayLink sometimes fails to
// start, stop and start again within a short duration.
type displayLink struct {
	callback func()
	// states is for starting or stopping the display link.
	states chan bool
	// done is closed when the display link is destroyed.
	done chan struct{}
	// dids receives the display id when the callback owner is moved
	// to a different screen.
	dids chan uint64
	// running tracks the desired state of the link. running is accessed
	// with atomic.
	running uint32
}

var mainFuncs = make(chan func(), 1)

// runOnMain runs the function on the main thread.
func runOnMain(f func()) {
	if C.isMainThread() {
		f()
		return
	}
	go func() {
		mainFuncs <- f
		C.gio_wakeupMainThread()
	}()
}

//export gio_dispatchMainFuncs
func gio_dispatchMainFuncs() {
	for {
		select {
		case f := <-mainFuncs:
			f()
		default:
			return
		}
	}
}

// nsstringToString converts a NSString to a Go string.
func nsstringToString(str C.CFTypeRef) string {
	if str == 0 {
		return ""
	}
	n := C.nsstringLength(str)
	if n == 0 {
		return ""
	}
	chars := make([]uint16, n)
	C.nsstringGetCharacters(str, (*C.unichar)(unsafe.Pointer(&chars[0])), 0, n)
	utf8 := utf16.Decode(chars)
	return string(utf8)
}

// stringToNSString converts a Go string to a retained NSString.
func stringToNSString(str string) C.CFTypeRef {
	u16 := utf16.Encode([]rune(str))
	var chars *C.unichar
	if len(u16) > 0 {
		chars = (*C.unichar)(unsafe.Pointer(&u16[0]))
	}
	return C.newNSString(chars, C.NSUInteger(len(u16)))
}

func NewDisplayLink(callback func()) (*displayLink, error) {
	d := &displayLink{
		callback: callback,
		done:     make(chan struct{}),
		states:   make(chan bool),
		dids:     make(chan uint64),
	}
	h := cgo.NewHandle(d)
	dl := C.gio_createDisplayLink(C.uintptr_t(h))
	if dl == 0 {
		return nil, errors.New("app: failed to create display link")
	}
	go d.run(dl, h)
	return d, nil
}

func (d *displayLink) run(dl C.CFTypeRef, h cgo.Handle) {
	defer C.gio_releaseDisplayLink(dl)
	defer h.Delete()
	var stopTimer *time.Timer
	var tchan <-chan time.Time
	started := false
	for {
		select {
		case <-tchan:
			tchan = nil
			started = false
			C.gio_stopDisplayLink(dl)
		case start := <-d.states:
			switch {
			case !start && tchan == nil:
				// stopTimeout is the delay before stopping the display link to
				// avoid the overhead of frequently starting and stopping the
				// link thread.
				const stopTimeout = 500 * time.Millisecond
				if stopTimer == nil {
					stopTimer = time.NewTimer(stopTimeout)
				} else {
					// stopTimer is always drained when tchan == nil.
					stopTimer.Reset(stopTimeout)
				}
				tchan = stopTimer.C
				atomic.StoreUint32(&d.running, 0)
			case start:
				if tchan != nil && !stopTimer.Stop() {
					<-tchan
				}
				tchan = nil
				atomic.StoreUint32(&d.running, 1)
				if !started {
					started = true
					C.gio_startDisplayLink(dl)
				}
			}
		case did := <-d.dids:
			C.gio_setDisplayLinkDisplay(dl, C.uint64_t(did))
		case <-d.done:
			return
		}
	}
}

func (d *displayLink) Start() {
	d.states <- true
}

func (d *displayLink) Stop() {
	d.states <- false
}

func (d *displayLink) Close() {
	close(d.done)
}

func (d *displayLink) SetDisplayID(did uint64) {
	d.dids <- did
}

//export gio_onFrameCallback
func gio_onFrameCallback(dl C.CFTypeRef, handle C.uintptr_t) {
	d := cgo.Handle(handle).Value().(*displayLink)
	if atomic.LoadUint32(&d.running) != 0 {
		d.callback()
	}
}

var macosCursorID = [...]byte{
	pointer.CursorDefault:                  0,
	pointer.CursorNone:                     1,
	pointer.CursorText:                     2,
	pointer.CursorVerticalText:             3,
	pointer.CursorPointer:                  4,
	pointer.CursorCrosshair:                5,
	pointer.CursorAllScroll:                6,
	pointer.CursorColResize:                7,
	pointer.CursorRowResize:                8,
	pointer.CursorGrab:                     9,
	pointer.CursorGrabbing:                 10,
	pointer.CursorNotAllowed:               11,
	pointer.CursorWait:                     12,
	pointer.CursorProgress:                 13,
	pointer.CursorNorthWestResize:          14,
	pointer.CursorNorthEastResize:          15,
	pointer.CursorSouthWestResize:          16,
	pointer.CursorSouthEastResize:          17,
	pointer.CursorNorthSouthResize:         18,
	pointer.CursorEastWestResize:           19,
	pointer.CursorWestResize:               20,
	pointer.CursorEastResize:               21,
	pointer.CursorNorthResize:              22,
	pointer.CursorSouthResize:              23,
	pointer.CursorNorthEastSouthWestResize: 24,
	pointer.CursorNorthWestSouthEastResize: 25,
}

// windowSetCursor updates the cursor from the current one to a new one
// and returns the new one.
func windowSetCursor(from, to pointer.Cursor) pointer.Cursor {
	if from == to {
		return to
	}
	if to == pointer.CursorNone {
		C.gio_hideCursor()
		return to
	}
	if from == pointer.CursorNone {
		C.gio_showCursor()
	}
	C.gio_setCursor(C.NSUInteger(macosCursorID[to]))
	return to
}

func (w *window) Wakeup() {
	runOnMain(func() {
		w.w.Event(wakeupEvent{})
	})
}
