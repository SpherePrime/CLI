package shell

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestBackgroundOutputRetainsBoundedTail(t *testing.T) {
	buffer := &syncBuffer{}
	var expected strings.Builder
	for index := range 40 {
		chunk := strings.Repeat(string(rune('a'+index%26)), 64*1024)
		expected.WriteString(chunk)
		written, err := buffer.WriteString(chunk)
		require.NoError(t, err)
		require.Equal(t, len(chunk), written)
	}
	written, err := buffer.Write([]byte("latest log line"))
	require.NoError(t, err)
	require.Equal(t, len("latest log line"), written)
	output := buffer.String()
	require.LessOrEqual(t, len(output), 1024*1024+128)
	require.Contains(t, output, "earlier output truncated")
	require.True(t, strings.HasSuffix(output, "latest log line"))
	expected.WriteString("latest log line")
	all := expected.String()
	require.Equal(t, "[earlier output truncated]\n"+all[len(all)-1024*1024:], output)
	_, err = buffer.Write([]byte(strings.Repeat("z", 2*1024*1024)))
	require.NoError(t, err)
	require.Equal(t, "[earlier output truncated]\n"+strings.Repeat("z", 1024*1024), buffer.String())
	require.True(t, strings.HasSuffix(output, "latest log line"))
	streamed := 0
	progress := &progressWriter{onProgress: func(chunk string) { streamed += len(chunk) }}
	written, err = progress.Write([]byte(all))
	require.NoError(t, err)
	require.Equal(t, len(all), written)
	require.Equal(t, len(all), streamed)
	require.Equal(t, "[earlier output truncated]\n"+all[len(all)-1024*1024:], progress.buf.String())
	unicodeOutput := &syncBuffer{}
	_, err = unicodeOutput.Write([]byte(strings.Repeat("я", 1024*1024/2) + "a"))
	require.NoError(t, err)
	require.True(t, utf8.ValidString(unicodeOutput.String()))
}
