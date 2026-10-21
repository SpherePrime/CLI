package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
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
}

func imageGenDescription() string {
	return renderTemplate(imageGenDescriptionTpl, imageGenDescriptionData{
		DefaultModel:  config.DefaultImageGenModel,
		DefaultMaxima: 10,
	})
}

type ImageGenParams struct {
	Prompt         string `json:"prompt" description:"Image description (required)"`
	Size           string `json:"size,omitempty" description:"Image size like 1024x1024, 1536x1024 or auto (provider default when omitted)"`
	Count          int    `json:"count,omitempty" description:"Images to generate, 1-10 (default 1)"`
	OutputDir      string `json:"output_dir,omitempty" description:"Save directory chosen for this task; without it images go to ./generated-images in the working directory"`
	FilenamePrefix string `json:"filename_prefix,omitempty" description:"File name prefix (default image)"`
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
	message := "Image generation is not configured: options.image_gen.api_key is empty. " +
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

func NewImageGenTool(
	permissions permission.Service,
	workingDir string,
	options *config.ImageGenOptions,
	resolver config.VariableResolver,
	client *http.Client,
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

			apiKey, err := resolveImageAPIKey(options, resolver)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if apiKey == "" {
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

			request := imageAPIRequest{
				Model:          options.GetModel(),
				Prompt:         params.Prompt,
				N:              count,
				Size:           strings.TrimSpace(params.Size),
				ResponseFormat: options.GetResponseFormat(),
			}

			images, err := generateImages(ctx, client, options.GetBaseURL(), apiKey, request, options.GetTimeout())
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
				report.WriteString("\n- " + path)
				if revised := strings.TrimSpace(images[index].RevisedPrompt); revised != "" && revised != params.Prompt {
					report.WriteString(fmt.Sprintf(" (revised prompt: %s)", revised))
				}
			}
			return fantasy.NewTextResponse(report.String()), nil
		},
	)
}
