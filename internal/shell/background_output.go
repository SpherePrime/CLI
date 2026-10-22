package shell

import "sync"

const backgroundOutputLimit = 1024 * 1024

type syncBuffer struct {
	mu        sync.RWMutex
	data      []byte
	start     int
	truncated bool
}

func (sb *syncBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	written := len(p)
	if len(p) >= backgroundOutputLimit {
		sb.truncated = sb.truncated || written+len(sb.data) > backgroundOutputLimit
		if cap(sb.data) != backgroundOutputLimit {
			sb.data = make([]byte, backgroundOutputLimit)
		} else {
			sb.data = sb.data[:backgroundOutputLimit]
		}
		copy(sb.data, p[len(p)-backgroundOutputLimit:])
		sb.start = 0
		return written, nil
	}
	if remaining := backgroundOutputLimit - len(sb.data); remaining > 0 {
		count := min(remaining, len(p))
		sb.data = append(sb.data, p[:count]...)
		p = p[count:]
	}
	if len(p) > 0 {
		sb.truncated = true
		first := copy(sb.data[sb.start:], p)
		copy(sb.data, p[first:])
		sb.start = (sb.start + len(p)) % backgroundOutputLimit
	}
	return written, nil
}

func (sb *syncBuffer) WriteString(value string) (int, error) {
	return sb.Write([]byte(value))
}

func (sb *syncBuffer) String() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	if !sb.truncated {
		return string(sb.data)
	}
	output := string(sb.data[sb.start:]) + string(sb.data[:sb.start])
	for len(output) > 0 && output[0]&0xc0 == 0x80 {
		output = output[1:]
	}
	return "[earlier output truncated]\n" + output
}
