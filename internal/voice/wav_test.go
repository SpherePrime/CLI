package voice

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestEncodeWAVHeader(t *testing.T) {
	t.Parallel()

	pcm := make([]byte, SampleRate*2)
	wav := encodeWAV(pcm)

	require.Equal(t, "RIFF", string(wav[:4]))
	require.Equal(t, "WAVE", string(wav[8:12]))
	require.Equal(t, "fmt ", string(wav[12:16]))
	require.Equal(t, uint32(headerSize-8+uint32(len(pcm))), binary.LittleEndian.Uint32(wav[4:8]))
	require.Equal(t, uint16(pcmFormat), binary.LittleEndian.Uint16(wav[20:22]))
	require.Equal(t, uint16(Channels), binary.LittleEndian.Uint16(wav[22:24]))
	require.Equal(t, uint32(SampleRate), binary.LittleEndian.Uint32(wav[24:28]))
	require.Equal(t, uint32(SampleRate*Channels*bytesPerSample), binary.LittleEndian.Uint32(wav[28:32]))
	require.Equal(t, "data", string(wav[36:40]))
	require.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(wav[40:44]))
	require.Equal(t, pcm, wav[headerSize:])
}

func TestDecodePCMRequiresAlignedSamples(t *testing.T) {
	t.Parallel()

	decoded, err := decodePCM([]byte{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2}, decoded)
}

func TestPcmDuration(t *testing.T) {
	t.Parallel()

	// One second of 16 kHz mono 16-bit audio is 32000 bytes.
	require.Equal(t, time.Second, pcmDuration(make([]byte, SampleRate*bytesPerSample)))
	require.Equal(t, 500*time.Millisecond, pcmDuration(make([]byte, SampleRate*bytesPerSample/2)))
}

func TestDecodePCMFromWAV(t *testing.T) {
	t.Parallel()

	pcm := []byte{0x01, 0x02, 0x03, 0x04}
	decoded, err := decodePCM(encodeWAV(pcm))
	require.NoError(t, err)
	require.Equal(t, pcm, decoded)
}

func TestDecodePCMWithUnpatchedHeader(t *testing.T) {
	t.Parallel()

	// A recorder killed before it can rewrite its sizes leaves 0xFFFFFFFF in
	// the header. The audio is still usable, so the real byte count wins.
	body := []byte{9, 8, 7, 6, 5, 4}
	header := []byte("RIFF\xff\xff\xff\xffWAVEfmt ")
	header = binary.LittleEndian.AppendUint32(header, 16)
	header = binary.LittleEndian.AppendUint16(header, pcmFormat)
	header = binary.LittleEndian.AppendUint16(header, Channels)
	header = binary.LittleEndian.AppendUint32(header, SampleRate)
	header = binary.LittleEndian.AppendUint32(header, SampleRate*2)
	header = binary.LittleEndian.AppendUint16(header, 2)
	header = binary.LittleEndian.AppendUint16(header, 16)
	header = append(header, []byte("data")...)
	header = binary.LittleEndian.AppendUint32(header, 0xffffffff)

	decoded, err := decodePCM(append(header, body...))
	require.NoError(t, err)
	require.Equal(t, body, decoded)
}

func TestDecodePCMSkipsLeadingChunks(t *testing.T) {
	t.Parallel()

	pcm := []byte{1, 2, 3, 4}
	buffer := []byte("RIFF")
	buffer = binary.LittleEndian.AppendUint32(buffer, uint32(4+8+8+8+len(pcm)))
	buffer = append(buffer, []byte("WAVE")...)
	// A LIST chunk before the audio, as written by some recorders.
	buffer = append(buffer, []byte("LIST")...)
	buffer = binary.LittleEndian.AppendUint32(buffer, 6)
	buffer = append(buffer, []byte("INFO")...)
	buffer = append(buffer, 0, 0)
	buffer = append(buffer, []byte("data")...)
	buffer = binary.LittleEndian.AppendUint32(buffer, uint32(len(pcm)))
	buffer = append(buffer, pcm...)

	decoded, err := decodePCM(buffer)
	require.NoError(t, err)
	require.Equal(t, pcm, decoded)
}

func TestDecodePCMRejectsHeaderWithoutData(t *testing.T) {
	t.Parallel()

	header := []byte("RIFF\x24\x00\x00\x00WAVEfmt ")
	_, err := decodePCM(header)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no data chunk")
}

func TestAudioWorthTranscribing(t *testing.T) {
	t.Parallel()

	// A quarter second of audio is shorter than the accidental-tap threshold.
	shortPCM := make([]byte, SampleRate*bytesPerSample/4)
	short := &Audio{WAV: encodeWAV(shortPCM), Duration: pcmDuration(shortPCM)}
	require.False(t, short.WorthTranscribing())

	longPCM := make([]byte, SampleRate*bytesPerSample*2)
	long := &Audio{WAV: encodeWAV(longPCM), Duration: pcmDuration(longPCM)}
	require.True(t, long.WorthTranscribing())

	require.False(t, (&Audio{}).WorthTranscribing())
	require.False(t, (*Audio)(nil).WorthTranscribing())
}
