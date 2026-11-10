Generate images and save them to disk. Uses the image API from the image settings; without one it uses an image model of a connected provider (for example a model whose name contains "image"). Calls POST {baseURL}/images/generations. Returns absolute paths of the saved files; handles both b64_json and url responses, the file extension is detected from content (png/jpg/webp). Default model of the image settings: {{ .DefaultModel }}.

The image model receives only the prompt and the source images you pass, never the conversation or other files. Write the prompt yourself from the task: what is shown, composition and framing, style that fits the project, colors and lighting, mood, any exact text to render. One image per idea; use count for variants.

To change an existing image, pass its path in source_images (up to 4) and describe the change in the prompt; the request goes to POST {baseURL}/images/edits. Do this when the user asks to change, fix, recolor, restyle, extend or combine an image, and on your own when the task needs a variant of an image that already exists (another color or size of a project asset, a matching icon, a fix to a generated image that came out wrong). Say what to keep as well as what to change ("keep the composition and background, make the cube blue"). Several source images can serve as references for style or content of a new image. Save the result as a new file unless the user asked to replace the original.

Set size whenever the task needs a specific shape: 1024x1024 square, 1536x1024 landscape, 1024x1536 portrait, or auto. Typical sizes: {{ .DefaultSizes }}. Omit size to use the provider default.

Decide where to save yourself: pass output_dir with a location that fits the task (for example an assets folder in the project); when the user names a folder, use it. Without output_dir the images go to ./generated-images in the working directory.

The images are already shown to the user in the chat. Do not read, view or verify the saved files afterwards; just report where they were saved and use them where the task needs them.

If the result says image generation is not configured, do not retry: tell the user that image generation needs setup and include this exact markdown link in your reply so the settings dialog opens on the right tab: [Image settings](prime://settings/image-generation)
