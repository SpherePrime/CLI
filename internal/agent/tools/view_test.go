package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/filetracker"
	"github.com/SpherePrime/CLI/internal/home"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestReadTextFileBoundaryCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.txt")

	var allLines []string
	for i := range 5 {
		allLines = append(allLines, fmt.Sprintf("line %d", i+1))
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(allLines, "\n")), 0o644))

	tests := []struct {
		name        string
		offset      int
		limit       int
		wantContent string
		wantHasMore bool
	}{
		{
			name:        "exactly limit lines remaining",
			offset:      0,
			limit:       5,
			wantContent: "line 1\nline 2\nline 3\nline 4\nline 5",
			wantHasMore: false,
		},
		{
			name:        "limit plus one line remaining",
			offset:      0,
			limit:       4,
			wantContent: "line 1\nline 2\nline 3\nline 4",
			wantHasMore: true,
		},
		{
			name:        "offset at last line",
			offset:      4,
			limit:       3,
			wantContent: "line 5",
			wantHasMore: false,
		},
		{
			name:        "offset beyond eof",
			offset:      10,
			limit:       3,
			wantContent: "",
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotContent, gotHasMore, err := readTextFile(filePath, tt.offset, tt.limit, 0)
			require.NoError(t, err)
			require.Equal(t, tt.wantContent, gotContent)
			require.Equal(t, tt.wantHasMore, gotHasMore)
		})
	}
}

func TestReadTextFileTruncatesLongLines(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "longline.txt")

	longLine := strings.Repeat("a", MaxLineLength+10)
	require.NoError(t, os.WriteFile(filePath, []byte(longLine), 0o644))

	content, hasMore, err := readTextFile(filePath, 0, 1, 0)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Equal(t, strings.Repeat("a", MaxLineLength)+"...", content)
}

func TestReadTextFileLineExceeding1MB(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "huge_line.txt")

	hugeLine := strings.Repeat("A", 2*1024*1024) // 2MB — exceeds bufio.Scanner max
	require.NoError(t, os.WriteFile(filePath, []byte(hugeLine), 0o644))

	content, hasMore, err := readTextFile(filePath, 0, 1, 0)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Equal(t, strings.Repeat("A", MaxLineLength)+"...", content)
}

func TestViewToolAllowsSmallSectionsOfLargeFiles(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "large.txt")
	lines := []string{strings.Repeat("a", MaxViewSize+1), "target line", "after target"}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644))

	tool := newViewToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp := runViewTool(t, tool, ctx, ViewParams{
		FilePath: filePath,
		Offset:   1,
		Limit:    1,
	})

	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "     2|target line")
	require.NotContains(t, resp.Content, "File is too large")

	var meta ViewResponseMetadata
	require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
	require.Equal(t, "target line", meta.Content)
}

func TestViewToolBlocksOversizedReturnedSections(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "large-section.txt")
	lines := make([]string, DefaultReadLimit)
	for i := range lines {
		lines[i] = strings.Repeat("a", MaxLineLength)
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644))

	tool := newViewToolForTest(workingDir)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp := runViewTool(t, tool, ctx, ViewParams{
		FilePath: filePath,
	})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Content section is too large")
}

func TestViewToolAllowsOversizedImages(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	ctx = context.WithValue(ctx, SupportsImagesContextKey, true)

	t.Run("valid PNG larger than MaxViewSize", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		filePath := filepath.Join(workingDir, "medium.png")
		pngBytes := encodeTestPNG(t, 280, 280, png.NoCompression, false)
		require.Greater(t, len(pngBytes), MaxViewSize)
		require.LessOrEqual(t, len(pngBytes), maxModelImageRawBytes)
		require.NoError(t, os.WriteFile(filePath, pngBytes, 0o644))

		tool := newViewToolForTest(workingDir)
		resp := runViewTool(t, tool, ctx, ViewParams{FilePath: filePath})
		require.False(t, resp.IsError)
		require.Equal(t, "image", resp.Type)
		require.Equal(t, "image/png", resp.MediaType)
		require.Equal(t, pngBytes, resp.Data)
	})

	t.Run("valid PNG larger than 3 MiB is fitted", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		filePath := filepath.Join(workingDir, "large.png")
		pngBytes := encodeTestPNG(t, 1200, 1200, png.NoCompression, false)
		require.Greater(t, len(pngBytes), maxModelImageRawBytes)
		require.NoError(t, os.WriteFile(filePath, pngBytes, 0o644))

		tool := newViewToolForTest(workingDir)
		resp := runViewTool(t, tool, ctx, ViewParams{FilePath: filePath})
		require.False(t, resp.IsError)
		require.Equal(t, "image", resp.Type)
		require.NotEmpty(t, resp.Data)
		require.LessOrEqual(t, len(resp.Data), maxModelImageRawBytes)

		cfg, _, err := image.DecodeConfig(bytes.NewReader(resp.Data))
		require.NoError(t, err)
		require.LessOrEqual(t, cfg.Width, maxModelImageDimension)
		require.LessOrEqual(t, cfg.Height, maxModelImageDimension)
	})

	t.Run("dummy oversized image bytes fall back untouched", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		filePath := filepath.Join(workingDir, "dummy.png")
		dummyBytes := bytes.Repeat([]byte("a"), MaxViewSize+1)
		require.NoError(t, os.WriteFile(filePath, dummyBytes, 0o644))

		tool := newViewToolForTest(workingDir)
		resp := runViewTool(t, tool, ctx, ViewParams{FilePath: filePath})
		require.False(t, resp.IsError)
		require.NotContains(t, resp.Content, "Image file is too large")
		require.Equal(t, "image", resp.Type)
		require.Equal(t, "image/png", resp.MediaType)
		require.Equal(t, dummyBytes, resp.Data)
	})
}

func TestFitImageForModel(t *testing.T) {
	t.Parallel()

	t.Run("keeps image within byte and dimension limits untouched", func(t *testing.T) {
		t.Parallel()

		smallPNG := encodeTestPNG(t, 64, 48, png.DefaultCompression, false)
		require.LessOrEqual(t, len(smallPNG), maxModelImageRawBytes)

		gotBytes, gotMime := fitImageForModel(smallPNG, "image/png")
		require.Equal(t, "image/png", gotMime)
		require.Equal(t, smallPNG, gotBytes)
	})

	t.Run("resizes image exceeding maxModelImageDimension", func(t *testing.T) {
		t.Parallel()

		widePNG := encodeTestPNG(t, maxModelImageDimension+500, 100, png.DefaultCompression, false)
		require.LessOrEqual(t, len(widePNG), maxModelImageRawBytes)

		gotBytes, gotMime := fitImageForModel(widePNG, "image/png")
		require.Equal(t, "image/png", gotMime)
		require.LessOrEqual(t, len(gotBytes), maxModelImageRawBytes)

		cfg, format, err := image.DecodeConfig(bytes.NewReader(gotBytes))
		require.NoError(t, err)
		require.Equal(t, "png", format)
		require.LessOrEqual(t, cfg.Width, maxModelImageDimension)
		require.LessOrEqual(t, cfg.Height, maxModelImageDimension)
		require.Equal(t, 2048, cfg.Width)
	})

	t.Run("falls back to JPEG when incompressible transparent PNG exceeds maxModelImageRawBytes", func(t *testing.T) {
		t.Parallel()

		noisyPNG := encodeTestPNG(t, 1400, 1400, png.BestSpeed, true)
		require.Greater(t, len(noisyPNG), maxModelImageRawBytes)

		gotBytes, gotMime := fitImageForModel(noisyPNG, "image/png")
		require.Equal(t, "image/jpeg", gotMime)
		require.NotEmpty(t, gotBytes)
		require.LessOrEqual(t, len(gotBytes), maxModelImageRawBytes)

		cfg, format, err := image.DecodeConfig(bytes.NewReader(gotBytes))
		require.NoError(t, err)
		require.Equal(t, "jpeg", format)
		require.Equal(t, 1400, cfg.Width)
		require.Equal(t, 1400, cfg.Height)
	})
}

func TestMCPResponseFitsImages(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), SupportsImagesContextKey, true)
	oversizedPNG := encodeTestPNG(t, 1200, 1200, png.NoCompression, false)
	require.Greater(t, len(oversizedPNG), maxModelImageRawBytes)

	t.Run("single image", func(t *testing.T) {
		t.Parallel()

		resp := mcpResponse(ctx, mcp.ToolResult{
			Type:      "image",
			Data:      oversizedPNG,
			MediaType: "image/png",
		})
		require.False(t, resp.IsError)
		require.Equal(t, "image", resp.Type)
		require.NotEmpty(t, resp.Data)
		require.LessOrEqual(t, len(resp.Data), maxModelImageRawBytes)
	})

	t.Run("multiple images", func(t *testing.T) {
		t.Parallel()

		resp := mcpResponse(ctx, mcp.ToolResult{
			Type: "image",
			Images: []mcp.ToolImage{
				{Data: oversizedPNG, MediaType: "image/png"},
				{Data: oversizedPNG, MediaType: "image/png"},
			},
		})
		require.False(t, resp.IsError)
		require.Equal(t, "image", resp.Type)
		require.Len(t, resp.Images, 2)
		for _, img := range resp.Images {
			require.NotEmpty(t, img.Data)
			require.LessOrEqual(t, len(img.Data), maxModelImageRawBytes)
		}
		require.LessOrEqual(t, len(resp.Data), maxModelImageRawBytes)
	})
}

func encodeTestPNG(t *testing.T, width, height int, level png.CompressionLevel, noisy bool) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	if noisy {
		state := uint32(0x12345678)
		for y := range height {
			rowStart := y * img.Stride
			for x := range width {
				state ^= state << 13
				state ^= state >> 17
				state ^= state << 5
				offset := rowStart + x*4
				img.Pix[offset] = uint8(state)
				img.Pix[offset+1] = uint8(state >> 8)
				img.Pix[offset+2] = uint8(state >> 16)
				img.Pix[offset+3] = uint8(128 + (state>>24)%128)
			}
		}
	} else {
		fill := color.NRGBA{R: 60, G: 140, B: 220, A: 255}
		for y := range height {
			rowStart := y * img.Stride
			for x := range width {
				offset := rowStart + x*4
				img.Pix[offset] = fill.R
				img.Pix[offset+1] = fill.G
				img.Pix[offset+2] = fill.B
				img.Pix[offset+3] = fill.A
			}
		}
	}

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: level}
	require.NoError(t, encoder.Encode(&buf, img))
	return buf.Bytes()
}

func TestReadTextFileEnforcesMaxContentSize(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "oversized.txt")
	lines := []string{
		strings.Repeat("a", MaxLineLength),
		strings.Repeat("b", MaxLineLength),
		"target line",
	}
	require.NoError(t, os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0o644))

	content, hasMore, err := readTextFile(filePath, 0, len(lines), MaxLineLength)
	require.ErrorAs(t, err, &contentTooLargeError{})
	require.Empty(t, content)
	require.False(t, hasMore)

	content, hasMore, err = readTextFile(filePath, 2, 1, MaxLineLength)
	require.NoError(t, err)
	require.Equal(t, "target line", content)
	require.False(t, hasMore)
}

func TestReadTextFileAllowsExactMaxContentSize(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	filePath := filepath.Join(workingDir, "exact-size.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("abcd\nefgh"), 0o644))

	content, hasMore, err := readTextFile(filePath, 0, 2, len("abcd\nefgh"))
	require.NoError(t, err)
	require.Equal(t, "abcd\nefgh", content)
	require.False(t, hasMore)
}

type mockViewPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockViewPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockViewPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *mockViewPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *mockViewPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *mockViewPermissionService) AutoApproveSession(sessionID string) {}

func (m *mockViewPermissionService) SetSkipRequests(skip bool) {}

func (m *mockViewPermissionService) SkipRequests() bool {
	return false
}

func (m *mockViewPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type mockFileTracker struct{}

func (m mockFileTracker) RecordRead(ctx context.Context, sessionID, path string) {}

func (m mockFileTracker) LastReadTime(ctx context.Context, sessionID, path string) time.Time {
	return time.Time{}
}

func (m mockFileTracker) ListReadFiles(ctx context.Context, sessionID string) ([]string, error) {
	return nil, nil
}

func newViewToolForTest(workingDir string) fantasy.AgentTool {
	permissions := &mockViewPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	return NewViewTool(nil, permissions, mockFileTracker{}, nil, workingDir)
}

func runViewTool(t *testing.T, tool fantasy.AgentTool, ctx context.Context, params ViewParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	call := fantasy.ToolCall{
		ID:    "test-call",
		Name:  ViewToolName,
		Input: string(input),
	}

	resp, err := tool.Run(ctx, call)
	require.NoError(t, err)
	return resp
}

var _ filetracker.Service = mockFileTracker{}

func TestReadBuiltinFile(t *testing.T) {
	t.Parallel()

	t.Run("reads prime-config skill", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "prime://skills/prime-config/SKILL.md",
		}, nil)
		require.NoError(t, err)
		require.NotEmpty(t, resp.Content)
		require.Contains(t, resp.Content, "Prime Configuration")
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "prime://skills/nonexistent/SKILL.md",
		}, nil)
		require.NoError(t, err)
		require.True(t, resp.IsError)
	})

	t.Run("metadata has skill info", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "prime://skills/prime-config/SKILL.md",
		}, nil)
		require.NoError(t, err)

		var meta ViewResponseMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.Equal(t, ViewResourceSkill, meta.ResourceType)
		require.Equal(t, "prime-config", meta.ResourceName)
		require.NotEmpty(t, meta.ResourceDescription)
	})

	t.Run("respects offset", func(t *testing.T) {
		t.Parallel()

		resp, err := readBuiltinFile(ViewParams{
			FilePath: "prime://skills/prime-config/SKILL.md",
			Offset:   5,
		}, nil)
		require.NoError(t, err)
		require.NotContains(t, resp.Content, "     1|")
		require.NotContains(t, resp.Content, "File has more lines")
	})

	t.Run("ignores limit on full read for prime-config and prime-designer", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			builtinPath string
			fsPath      string
		}{
			{
				builtinPath: "prime://skills/prime-config/SKILL.md",
				fsPath:      "builtin/prime-config/SKILL.md",
			},
			{
				builtinPath: "prime://skills/prime-designer/SKILL.md",
				fsPath:      "builtin/prime-designer/SKILL.md",
			},
			{
				builtinPath: "prime://skills/prime-designer/references/quality-gate.md",
				fsPath:      "builtin/prime-designer/references/quality-gate.md",
			},
		} {
			raw, err := skills.BuiltinFS().ReadFile(tc.fsPath)
			require.NoError(t, err)

			resp, err := readBuiltinFile(ViewParams{
				FilePath: tc.builtinPath,
				Offset:   0,
				Limit:    200,
			}, nil)
			require.NoError(t, err)
			require.False(t, resp.IsError)
			require.NotContains(t, resp.Content, "File has more lines")

			var meta ViewResponseMetadata
			require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
			require.Equal(t, string(raw), meta.Content)
		}
	})
}

func TestSniffImageMimeType(t *testing.T) {
	t.Parallel()

	jpegMagic := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pngMagic := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	gifMagic := []byte("GIF89a")
	// Minimal RIFF/WEBP header.
	webpMagic := append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 16)...)
	random := []byte("not an image at all, just text")

	cases := []struct {
		name     string
		data     []byte
		fallback string
		want     string
	}{
		{"jpeg bytes in .png file uses sniffed", jpegMagic, "image/png", "image/jpeg"},
		{"png bytes in .jpg file uses sniffed", pngMagic, "image/jpeg", "image/png"},
		{"gif bytes uses sniffed", gifMagic, "image/png", "image/gif"},
		{"webp bytes uses sniffed", webpMagic, "image/png", "image/webp"},
		{"matching extension and content keeps sniffed", pngMagic, "image/png", "image/png"},
		{"unsniffable content falls back", random, "image/png", "image/png"},
		{"empty content falls back", nil, "image/jpeg", "image/jpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, sniffImageMimeType(tc.data, tc.fallback))
		})
	}
}

func TestReadBuiltinSkillGivesDiskDirectoryForResources(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("PRIME_GLOBAL_DATA", dataRoot)

	resp, err := readBuiltinFile(ViewParams{FilePath: "prime://skills/prime-designer/SKILL.md"}, nil)
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Prime Designer")
	directory := filepath.Join(dataRoot, "builtin-skills", "prime-designer")
	require.Contains(t, resp.Content, "<skill_directory>"+filepath.ToSlash(directory)+"</skill_directory>")
	require.FileExists(t, filepath.Join(directory, "scripts", "audit.mjs"))

	resp, err = readBuiltinFile(ViewParams{FilePath: "prime://skills/prime-designer/references/quality-gate.md"}, nil)
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.NotContains(t, resp.Content, "<skill_directory>")

	resp, err = readBuiltinFile(ViewParams{FilePath: "prime://skills/prime-config/SKILL.md"}, nil)
	require.NoError(t, err)
	require.NotContains(t, resp.Content, "<skill_directory>")
}

type denyRecordingPermissionService struct {
	*mockViewPermissionService
	called bool
}

func (d *denyRecordingPermissionService) Request(context.Context, permission.CreatePermissionRequest) (bool, error) {
	d.called = true
	return false, nil
}

func TestViewToolReadsLocalSkillsCompletely(t *testing.T) {
	makeSkillContent := func(totalLines int, longLineLen int) string {
		lines := make([]string, totalLines)
		lines[0] = "---"
		lines[1] = "name: custom-skill"
		lines[2] = "description: Test skill"
		lines[3] = "---"
		lines[4] = strings.Repeat("L", longLineLen)
		for i := 5; i < totalLines; i++ {
			lines[i] = fmt.Sprintf("instruction line %d", i+1)
		}
		return strings.Join(lines, "\n")
	}

	t.Run("project SKILL.md and skills directory references ignore limit and long line cap", func(t *testing.T) {
		workingDir := t.TempDir()
		skillDir := filepath.Join(workingDir, "skills", "custom-skill")
		refsDir := filepath.Join(skillDir, "references")
		require.NoError(t, os.MkdirAll(refsDir, 0o755))

		skillContent := makeSkillContent(260, MaxLineLength+500)
		skillPath := filepath.Join(skillDir, "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte(skillContent), 0o644))

		refContent := makeSkillContent(260, MaxLineLength+300)
		refPath := filepath.Join(refsDir, "guide.md")
		require.NoError(t, os.WriteFile(refPath, []byte(refContent), 0o644))

		tool := newViewToolForTest(workingDir)
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

		for _, tc := range []struct {
			path    string
			content string
		}{
			{skillPath, skillContent},
			{refPath, refContent},
		} {
			resp := runViewTool(t, tool, ctx, ViewParams{
				FilePath: tc.path,
				Offset:   0,
				Limit:    200,
			})
			require.False(t, resp.IsError)
			require.NotContains(t, resp.Content, "File has more lines")
			require.NotContains(t, resp.Content, "...")

			var meta ViewResponseMetadata
			require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
			require.Equal(t, tc.content, meta.Content)
		}
	})

	t.Run("env var expanded skillsPaths skips permission and reads full large skill", func(t *testing.T) {
		workingDir := t.TempDir()
		skillsRoot := t.TempDir()
		skillDir := filepath.Join(skillsRoot, "env-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))

		skillContent := makeSkillContent(300, MaxViewSize+1024)
		skillPath := filepath.Join(skillDir, "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte(skillContent), 0o644))

		t.Setenv("PRIME_TEST_SKILLS_DIR", skillsRoot)
		perms := &denyRecordingPermissionService{
			mockViewPermissionService: &mockViewPermissionService{
				Broker: pubsub.NewBroker[permission.PermissionRequest](),
			},
		}
		tool := NewViewTool(nil, perms, mockFileTracker{}, nil, workingDir, "$PRIME_TEST_SKILLS_DIR")
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

		resp := runViewTool(t, tool, ctx, ViewParams{
			FilePath: skillPath,
			Offset:   0,
			Limit:    200,
		})
		require.False(t, resp.IsError)
		require.False(t, perms.called)
		require.NotContains(t, resp.Content, "File has more lines")

		var meta ViewResponseMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.Equal(t, skillContent, meta.Content)
	})

	t.Run("tilde expanded skillsPaths skips permission and reads full skill", func(t *testing.T) {
		require.Equal(t, filepath.Join(home.Dir(), "custom-skills"), expandSkillsPath("~/custom-skills"))

		if home.Dir() == "" {
			t.Skip("no home directory")
		}
		skillsRoot, err := os.MkdirTemp(home.Dir(), "prime-view-skill-test-*")
		if err != nil {
			t.Skipf("home directory not writable: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(skillsRoot) })

		skillDir := filepath.Join(skillsRoot, "tilde-skill")
		require.NoError(t, os.MkdirAll(skillDir, 0o755))

		skillContent := makeSkillContent(260, MaxLineLength+400)
		skillPath := filepath.Join(skillDir, "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte(skillContent), 0o644))

		workingDir := t.TempDir()
		perms := &denyRecordingPermissionService{
			mockViewPermissionService: &mockViewPermissionService{
				Broker: pubsub.NewBroker[permission.PermissionRequest](),
			},
		}
		tool := NewViewTool(nil, perms, mockFileTracker{}, nil, workingDir, home.Short(skillsRoot))
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

		resp := runViewTool(t, tool, ctx, ViewParams{
			FilePath: skillPath,
			Offset:   0,
			Limit:    200,
		})
		require.False(t, resp.IsError)
		require.False(t, perms.called)

		var meta ViewResponseMetadata
		require.NoError(t, json.Unmarshal([]byte(resp.Metadata), &meta))
		require.Equal(t, skillContent, meta.Content)
	})

	t.Run("outside SKILL.md not in skillsPaths still requires permission", func(t *testing.T) {
		workingDir := t.TempDir()
		outsideDir := t.TempDir()
		skillPath := filepath.Join(outsideDir, "SKILL.md")
		require.NoError(t, os.WriteFile(skillPath, []byte("outside"), 0o644))

		perms := &denyRecordingPermissionService{
			mockViewPermissionService: &mockViewPermissionService{
				Broker: pubsub.NewBroker[permission.PermissionRequest](),
			},
		}
		tool := NewViewTool(nil, perms, mockFileTracker{}, nil, workingDir)
		ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")

		resp := runViewTool(t, tool, ctx, ViewParams{FilePath: skillPath})
		require.True(t, perms.called)
		require.True(t, resp.IsError)
	})
}
