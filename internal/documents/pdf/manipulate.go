package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/documents/docutil"
	"github.com/SpherePrime/CLI/vendordeps/pdfcpu/pdfcpu/pkg/api"
	"github.com/SpherePrime/CLI/vendordeps/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func relaxedConfig() *model.Configuration {
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	return conf
}

func Merge(inputs []string, outputPath string) (*Result, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one input PDF is required")
	}
	resolvedOut, err := resolvePath("", outputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid output_path: %w", err)
	}

	readers := make([]io.ReadSeeker, 0, len(inputs))
	for i, in := range inputs {
		resolvedIn, err := resolvePath("", in)
		if err != nil {
			return nil, fmt.Errorf("invalid input %d: %w", i, err)
		}
		raw, err := os.ReadFile(resolvedIn)
		if err != nil {
			return nil, fmt.Errorf("read input %s: %w", resolvedIn, err)
		}
		readers = append(readers, bytes.NewReader(raw))
	}

	bgCtx := context.Background()
	var buf bytes.Buffer
	if err := api.MergeRaw(bgCtx, readers, &buf, false, relaxedConfig()); err != nil {
		return nil, fmt.Errorf("merge pdfs: %w", err)
	}

	outBytes := buf.Bytes()
	if err := docutil.WriteFileAtomic(resolvedOut, outBytes); err != nil {
		return nil, fmt.Errorf("write merged pdf: %w", err)
	}

	pageCount, err := api.PageCount(bgCtx, bytes.NewReader(outBytes), relaxedConfig())
	if err != nil {
		return nil, fmt.Errorf("count merged pages: %w", err)
	}

	return &Result{
		Path:      resolvedOut,
		PageCount: pageCount,
		SizeBytes: int64(len(outBytes)),
	}, nil
}

func Split(inputPath string, pageRange string, span int, outputPath string, outputDir string) (*SplitResult, error) {
	resolvedIn, err := resolvePath("", inputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid input_path: %w", err)
	}
	raw, err := os.ReadFile(resolvedIn)
	if err != nil {
		return nil, fmt.Errorf("read input %s: %w", resolvedIn, err)
	}

	bgCtx := context.Background()
	trimmedRange := strings.TrimSpace(pageRange)
	if trimmedRange != "" {
		var targetPath string
		switch {
		case strings.TrimSpace(outputPath) != "":
			targetPath, err = resolvePath("", outputPath)
			if err != nil {
				return nil, fmt.Errorf("invalid output_path: %w", err)
			}
		case strings.TrimSpace(outputDir) != "":
			resolvedDir, dirErr := resolvePath("", outputDir)
			if dirErr != nil {
				return nil, fmt.Errorf("invalid output_dir: %w", dirErr)
			}
			base := strings.TrimSuffix(filepath.Base(resolvedIn), filepath.Ext(resolvedIn))
			targetPath = filepath.Join(resolvedDir, base+"_pages.pdf")
		default:
			return nil, errors.New("output_path or output_dir is required")
		}

		var buf bytes.Buffer
		if err := api.Trim(bgCtx, bytes.NewReader(raw), &buf, parsePageSelection(trimmedRange), relaxedConfig()); err != nil {
			return nil, fmt.Errorf("extract pages %q: %w", trimmedRange, err)
		}
		outBytes := buf.Bytes()
		if len(outBytes) == 0 {
			return nil, fmt.Errorf("page_range %q matched no pages", trimmedRange)
		}
		if err := docutil.WriteFileAtomic(targetPath, outBytes); err != nil {
			return nil, fmt.Errorf("write split pdf: %w", err)
		}
		pageCount, err := api.PageCount(bgCtx, bytes.NewReader(outBytes), relaxedConfig())
		if err != nil {
			return nil, fmt.Errorf("count split pages: %w", err)
		}
		return &SplitResult{
			Files: []Result{{
				Path:      targetPath,
				PageCount: pageCount,
				SizeBytes: int64(len(outBytes)),
			}},
		}, nil
	}

	if span <= 0 {
		return nil, fmt.Errorf("span must be >= 1 when page_range is empty, got %d", span)
	}
	resolvedDir, err := resolvePath("", outputDir)
	if err != nil {
		return nil, fmt.Errorf("invalid output_dir: %w", err)
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output_dir: %w", err)
	}

	spans, err := api.SplitRaw(bgCtx, bytes.NewReader(raw), span, relaxedConfig())
	if err != nil {
		return nil, fmt.Errorf("split pdf: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(resolvedIn), filepath.Ext(resolvedIn))
	files := make([]Result, 0, len(spans))
	for _, ps := range spans {
		data, err := io.ReadAll(ps.Reader)
		if err != nil {
			return nil, fmt.Errorf("read split span %d-%d: %w", ps.From, ps.Thru, err)
		}
		name := base + "_" + strconv.Itoa(ps.From)
		if ps.From != ps.Thru {
			name += "-" + strconv.Itoa(ps.Thru)
		}
		outFile := filepath.Join(resolvedDir, name+".pdf")
		if err := docutil.WriteFileAtomic(outFile, data); err != nil {
			return nil, fmt.Errorf("write split file %s: %w", outFile, err)
		}
		files = append(files, Result{
			Path:      outFile,
			PageCount: ps.Thru - ps.From + 1,
			SizeBytes: int64(len(data)),
		})
	}

	return &SplitResult{Files: files}, nil
}

func Rotate(inputPath string, pageRange string, angle int, outputPath string) (*Result, error) {
	resolvedIn, err := resolvePath("", inputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid input_path: %w", err)
	}
	resolvedOut, err := resolvePath("", outputPath)
	if err != nil {
		return nil, fmt.Errorf("invalid output_path: %w", err)
	}
	if angle%90 != 0 {
		return nil, fmt.Errorf("angle must be a multiple of 90, got %d", angle)
	}

	raw, err := os.ReadFile(resolvedIn)
	if err != nil {
		return nil, fmt.Errorf("read input %s: %w", resolvedIn, err)
	}

	bgCtx := context.Background()
	var buf bytes.Buffer
	if err := api.Rotate(bgCtx, bytes.NewReader(raw), &buf, angle, parsePageSelection(pageRange), relaxedConfig()); err != nil {
		return nil, fmt.Errorf("rotate pdf: %w", err)
	}

	outBytes := buf.Bytes()
	if err := docutil.WriteFileAtomic(resolvedOut, outBytes); err != nil {
		return nil, fmt.Errorf("write rotated pdf: %w", err)
	}

	pageCount, err := api.PageCount(bgCtx, bytes.NewReader(outBytes), relaxedConfig())
	if err != nil {
		return nil, fmt.Errorf("count rotated pages: %w", err)
	}

	return &Result{
		Path:      resolvedOut,
		PageCount: pageCount,
		SizeBytes: int64(len(outBytes)),
	}, nil
}
