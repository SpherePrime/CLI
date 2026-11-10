package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

//go:embed imagegen.md.tpl
var imageGenDescriptionTmpl []byte

var imageGenDescriptionTpl = template.Must(
	template.New("imageGenDescription").
		Parse(string(imageGenDescriptionTmpl)),
)

type imageGenDescriptionData struct {
	DefaultModel  string
	DefaultMaxima int
	DefaultSizes  string
}

func imageGenDescription() string {
	return renderTemplate(imageGenDescriptionTpl, imageGenDescriptionData{
		DefaultModel:  config.DefaultImageGenModel,
		DefaultMaxima: 10,
		DefaultSizes:  "1024x1024, 1536x1024, 1024x1536, auto",
	})
}

type ImageGenParams struct {
	Prompt         string `json:"prompt" description:"Image description (required)"`
	Size           string `json:"size,omitempty" description:"Image size like 1024x1024, 1536x1024 or auto (provider default when omitted)"`
	Count          int    `json:"count,omitempty" description:"Images to generate, 1-10 (default 1)"`
	OutputDir      string `json:"output_dir,omitempty" description:"Save directory chosen for this task; without it images go to ./generated-images in the working directory"`
	FilenamePrefix string   `json:"filename_prefix,omitempty" description:"File name prefix (default image)"`
	SourceImages   []string `json:"source_images,omitempty" description:"Paths of existing images (up to 4) to edit or to use as references; when set, the edits endpoint is used and the prompt describes the change"`
}

type ImageGenPermissionsParams struct {
	Prompt    string `json:"prompt"`
	OutputDir string `json:"output_dir"`
	Count     int    `json:"count,omitempty"`
}

const (
	ImageGenToolName = "generate_image"
	// ImageGenSettingsLink is the markdown link the tool asks the agent to
	// repeat when image generation is not configured. The desktop chat opens
	// the settings dialog on the image tab for prime:// links.
	ImageGenSettingsLink = "[Image settings](prime://settings/image-generation)"
)

// imageGenNotConfiguredResponse explains what is missing and tells the agent
// to guide the user to the settings instead of retrying.
func imageGenNotConfiguredResponse() fantasy.ToolResponse {
	message := "Image generation is not configured: options.image_gen.api_key is empty and no connected provider has an image model. " +
		"Tell the user that image generation needs setup and include this exact markdown link in your reply: " +
		ImageGenSettingsLink + " " +
		"The link opens the settings dialog on the image generation tab where the API key, model and base URL are set. " +
		"Do not call generate_image again until the user has saved the settings."
	return fantasy.NewTextErrorResponse(message)
}

// resolveImageAPIKey resolves a $VAR reference in the configured API key at
// call time so updated environment variables are picked up without a restart.
func resolveImageAPIKey(options *config.ImageGenOptions, resolver config.VariableResolver) (string, error) {
	if options == nil || strings.TrimSpace(options.APIKey) == "" {
		return "", nil
	}
	value := strings.TrimSpace(options.APIKey)
	if resolver == nil {
		return value, nil
	}
	resolved, err := resolver.ResolveValue(value)
	if err != nil {
		return "", fmt.Errorf("failed to resolve options.image_gen.api_key: %w", err)
	}
	return strings.TrimSpace(resolved), nil
}

// ProviderImageModelFunc returns an image model of a connected provider, used
// when options.image_gen has no API key of its own.
type ProviderImageModelFunc func() (config.ProviderImageModel, bool)

func NewImageGenTool(
	permissions permission.Service,
	workingDir string,
	options *config.ImageGenOptions,
	resolver config.VariableResolver,
	client *http.Client,
) fantasy.AgentTool {
	return NewImageGenToolWithProviders(permissions, workingDir, options, resolver, client, nil)
}

// imageTarget is the endpoint, key and model one generate_image call uses.
type imageTarget struct {
	baseURL        string
	apiKey         string
	model          string
	responseFormat string
}

func resolveImageTarget(options *config.ImageGenOptions, resolver config.VariableResolver, providerModel ProviderImageModelFunc) (imageTarget, bool, error) {
	apiKey, err := resolveImageAPIKey(options, resolver)
	if err != nil {
		return imageTarget{}, false, err
	}
	if apiKey != "" {
		return imageTarget{baseURL: options.GetBaseURL(), apiKey: apiKey, model: options.GetModel(), responseFormat: options.GetResponseFormat()}, true, nil
	}
	if providerModel == nil {
		return imageTarget{}, false, nil
	}
	found, ok := providerModel()
	if !ok {
		return imageTarget{}, false, nil
	}
	key := strings.TrimSpace(found.APIKey)
	if key != "" && resolver != nil {
		resolved, err := resolver.ResolveValue(key)
		if err != nil {
			return imageTarget{}, false, fmt.Errorf("failed to resolve the API key of provider %s: %w", found.ProviderID, err)
		}
		key = strings.TrimSpace(resolved)
	}
	return imageTarget{baseURL: found.BaseURL, apiKey: key, model: found.Model}, true, nil
}

func NewImageGenToolWithProviders(
	permissions permission.Service,
	workingDir string,
	options *config.ImageGenOptions,
	resolver config.VariableResolver,
	client *http.Client,
	providerModel ProviderImageModelFunc,
) fantasy.AgentTool {
	if client == nil {
		client = &http.Client{}
	}
	return fantasy.NewParallelAgentTool(
		ImageGenToolName,
		imageGenDescription(),
		func(ctx context.Context, params ImageGenParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Prompt) == "" {
				return fantasy.NewTextErrorResponse("prompt is required"), nil
			}

			target, configured, err := resolveImageTarget(options, resolver, providerModel)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if !configured {
				return imageGenNotConfiguredResponse(), nil
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}
			if count > 10 {
				count = 10
			}

			outputDir := params.OutputDir
			if strings.TrimSpace(outputDir) == "" {
				outputDir = filepath.Join(workingDir, "generated-images")
			}
			if !filepath.IsAbs(outputDir) {
				outputDir = filepath.Join(workingDir, outputDir)
			}
			outputDir = filepath.Clean(outputDir)

			filenamePrefix := params.FilenamePrefix
			if strings.TrimSpace(filenamePrefix) == "" {
				filenamePrefix = "image"
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for generating images")
			}

			allowed, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        outputDir,
				ToolName:    ImageGenToolName,
				Action:      "write",
				Description: fmt.Sprintf("Generate %d image(s) for prompt %q and save to %s", count, params.Prompt, outputDir),
				Params:      ImageGenPermissionsParams{Prompt: params.Prompt, OutputDir: outputDir, Count: count},
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !allowed {
				return NewPermissionDeniedResponse(), nil
			}

			sources, err := readSourceImages(workingDir, params.SourceImages)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			request := imageAPIRequest{
				Model:          target.model,
				Prompt:         params.Prompt,
				N:              count,
				Size:           strings.TrimSpace(params.Size),
				ResponseFormat: target.responseFormat,
			}

			images, err := generateImages(ctx, client, target.baseURL, target.apiKey, request, sources, options.GetTimeout())
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to generate images: " + err.Error()), nil
			}

			paths, err := saveImages(images, outputDir, filenamePrefix)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}

			var report strings.Builder
			report.WriteString(fmt.Sprintf("Saved %d image(s):", len(paths)))
			for index, path := range paths {
				fmt.Fprintf(&report, "\n- %s", path)
				if revised := strings.TrimSpace(images[index].RevisedPrompt); revised != "" && revised != params.Prompt {
					fmt.Fprintf(&report, " (revised prompt: %s)", revised)
				}
			}

			// Every image travels to clients so the chat can show them all;
			// providers still see a single media item, the rest only enrich
			// the UI. The file list stays in the text content for the model.
			payloads := make([]fantasy.ImagePayload, len(images))
			for index, image := range images {
				payloads[index] = fantasy.ImagePayload{
					Data:      image.Bytes,
					MediaType: imageMimeType(image.Bytes),
					Path:      paths[index],
				}
			}
			return fantasy.NewImagesResponse(payloads, report.String()), nil
		},
	)
}

func imageMimeType(bytes []byte) string {
	switch detectImageExtension(bytes) {
	case "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

const (
	maxSourceImages     = 4
	maxSourceImageBytes = 20 << 20
)

func readSourceImages(workingDir string, paths []string) ([]sourceImage, error) {
	if len(paths) > maxSourceImages {
		return nil, fmt.Errorf("source_images accepts up to %d images", maxSourceImages)
	}
	sources := make([]sourceImage, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(workingDir, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("source image %s: %w", path, err)
		}
		if info.Size() > maxSourceImageBytes {
			return nil, fmt.Errorf("source image %s is larger than 20 MB", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("source image %s: %w", path, err)
		}
		if !isKnownImage(data) {
			return nil, fmt.Errorf("source image %s is not a png, jpeg or webp image", path)
		}
		sources = append(sources, sourceImage{Name: filepath.Base(path), MediaType: imageMimeType(data), Bytes: data})
	}
	return sources, nil
}

func isKnownImage(data []byte) bool {
	isPNG := len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n"
	isJPEG := len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
	isWebP := len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	return isPNG || isJPEG || isWebP
}
