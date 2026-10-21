Generate images through the configured OpenAI-compatible image API (POST {baseURL}/images/generations) and save them to disk. Returns absolute paths of the saved files; handles both b64_json and url responses, the file extension is detected from content (png/jpg/webp). Default model: {{ .DefaultModel }}.

If the result says image generation is not configured, do not retry: tell the user that image generation needs setup and include this exact markdown link in your reply so the settings dialog opens on the right tab: [Image settings](prime://settings/image-generation)
