Generate images through the configured OpenAI-compatible image API (POST {baseURL}/images/generations) and save them to disk. Returns absolute paths of the saved files; handles both b64_json and url responses, the file extension is detected from content (png/jpg/webp). Default model: {{ .DefaultModel }}.

Set size whenever the task needs a specific size: 1024x1024 square, 1536x1024 landscape, 1024x1536 portrait, or auto. Typical sizes for this model: {{ .DefaultSizes }}. Omit size to use the provider default.

Decide where to save yourself: pass output_dir with a location that fits the task (for example an assets folder in the project); when the user names a folder, use it. Without output_dir the images go to ./generated-images in the working directory.

The images are already shown to the user in the chat. Do not read, view or verify the saved files afterwards; just report where they were saved.

If the result says image generation is not configured, do not retry: tell the user that image generation needs setup and include this exact markdown link in your reply so the settings dialog opens on the right tab: [Image settings](prime://settings/image-generation)
