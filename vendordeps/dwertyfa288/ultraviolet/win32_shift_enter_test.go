//go:build windows

package uv

import (
	"bytes"
	"encoding/binary"
	"testing"

	xwindows "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/windows"
)

// keyEventRecord builds an INPUT_RECORD the way ReadConsoleInput returns one:
// the typed event fields are read back through a 16-byte little-endian blob,
// so the test packs the fields into the same layout the serializer consumes.
func keyEventRecord(t *testing.T, keyDown, repeatCount uint16, vk, sc uint16, char rune, cks uint32) xwindows.InputRecord {
	t.Helper()

	var rec xwindows.InputRecord
	rec.EventType = xwindows.KEY_EVENT
	binary.LittleEndian.PutUint32(rec.Event[0:4], uint32(keyDown))
	binary.LittleEndian.PutUint16(rec.Event[4:6], repeatCount)
	binary.LittleEndian.PutUint16(rec.Event[6:8], vk)
	binary.LittleEndian.PutUint16(rec.Event[8:10], sc)
	binary.LittleEndian.PutUint16(rec.Event[10:12], uint16(char))
	binary.LittleEndian.PutUint32(rec.Event[12:16], cks)
	return rec
}

// TestWin32ShiftEnterRoundTrip proves the VT-mode encoder fix end to end: a
// console KEY_EVENT_RECORD for a shifted, non-printable key must survive the
// serialize -> decode hop with its modifier intact, so that key matching sees
// "shift+enter" instead of a plain "enter" that would send the message.
func TestWin32ShiftEnterRoundTrip(t *testing.T) {
	r := NewTerminalReader(nil, "")
	r.vtInput = true

	var buf bytes.Buffer
	r.serializeWin32InputRecords([]xwindows.InputRecord{
		// VK_RETURN (13), scan code 28, Char 0x0D (CR), key down, shift held.
		keyEventRecord(t, 1, 1, 13, 28, 0x0D, xwindows.SHIFT_PRESSED),
	}, &buf)

	n, ev := r.Decode(buf.Bytes())
	if n != buf.Len() {
		t.Fatalf("decoded %d of %d bytes: %q", n, buf.Len(), buf.String())
	}
	kp, ok := ev.(KeyPressEvent)
	if !ok {
		t.Fatalf("event is %T, want KeyPressEvent", ev)
	}
	if !kp.MatchString("shift+enter") {
		t.Fatalf("decoded key is %q with mod=%v, want the matcher to see \"shift+enter\"", kp.String(), kp.Mod)
	}

	// ctrl+shift+space rides the same fallback path (space is a control
	// character), which the voice binding depends on.
	buf.Reset()
	r.serializeWin32InputRecords([]xwindows.InputRecord{
		keyEventRecord(t, 1, 1, 32, 49, 32, xwindows.SHIFT_PRESSED|xwindows.LEFT_CTRL_PRESSED),
	}, &buf)
	_, ev = r.Decode(buf.Bytes())
	kp, ok = ev.(KeyPressEvent)
	if !ok {
		t.Fatalf("event is %T, want KeyPressEvent", ev)
	}
	if !kp.MatchString("ctrl+shift+space") {
		t.Fatalf("decoded key is %q, want the matcher to see \"ctrl+shift+space\"", kp.String())
	}
}

// TestWin32PrintableKeystrokesStayOnTheRunePath guards the other half of the
// fix: printable keys must keep the fast rune path. Re-encoding them through
// the Win32 escape would drop the shift on symbols (shift+1 is "!", not "1"),
// so the serializer has to emit the bare rune there.
func TestWin32PrintableKeystrokesStayOnTheRunePath(t *testing.T) {
	r := NewTerminalReader(nil, "")
	r.vtInput = true

	cases := []struct {
		char rune
		cks  uint32
		want string
	}{
		{'a', 0, "a"},                        // plain press
		{'A', xwindows.SHIFT_PRESSED, "A"},    // shifted letter
		{'!', xwindows.SHIFT_PRESSED, "!"},    // shifted symbol: shift lives in the rune
		// Enter: vk 13 (VK_RETURN), Char 0x0D is a control char, shift held.
		// This is the case the whole feature exists for - it must take the
		// Win32 escape so the shift survives to the matcher as "shift+enter".
		{13, xwindows.SHIFT_PRESSED, "shift+enter"},
	}

	for i, c := range cases {
		var buf bytes.Buffer
		r.serializeWin32InputRecords([]xwindows.InputRecord{
			keyEventRecord(t, 1, 1, uint16(c.char), 45, c.char, c.cks),
		}, &buf)

		if i < 3 {
			// Printable keys stay a single rune; the escape prefix is the
			// tell-tale of the Win32 fallback path.
			if len(buf.Bytes()) != 1 || buf.Bytes()[0] == 0x1b {
				t.Fatalf("case %d: serialized %q, want a single bare rune %q", i, buf.Bytes(), c.char)
			}
		}

		n, ev := r.Decode(buf.Bytes())
		if n != buf.Len() {
			t.Fatalf("case %d: decoded %d of %d bytes", i, n, buf.Len())
		}
		kp, ok := ev.(KeyPressEvent)
		if !ok {
			t.Fatalf("case %d: event is %T, want KeyPressEvent", i, ev)
		}
		if got := kp.String(); got != c.want {
			t.Fatalf("case %d: decoded key is %q, want %q", i, got, c.want)
		}
	}
}
