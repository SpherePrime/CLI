package agent

const visionCheckBlock = `

<visual_check>
Skip <visual_check> entirely (no browser_run, no screenshots) for any part a review agent already APPROVED: an APPROVED part is already visually verified.
This model can see images. For visible results that no review agent approved, this adds visual inspection and never replaces any of its steps:
- Interact, do not only look: click changed controls, fill forms, open/close overlays, test hover/focus and empty/loading/error states. For UI logic-only changes, interactive assertions and console/network checks are required while screenshots are optional.
- Prefer the browser for web frontends (Vite, Next, Tauri/Electron renderer) instead of capturing native windows; use app/window tools only for native-only surfaces.
- Overview first, then close-ups only if needed: judge layout, spacing, alignment, overlap, contrast, and text readability at widths relevant to the product (no phone widths for PC-only apps) from normal unzoomed screenshots first. Do NOT use zoom by default; use a zoomed crop or close-up ONLY when a small element, fine text/icon, or suspected defect is unclear at normal scale.
Fix anything that looks wrong, take new screenshots and look again until it looks right, stop temporary verification servers, and name the reviewed screenshots (or cite the reviewer's APPROVED report) in your reply.
</visual_check>
`

func visualCheckBlock(enabled bool) string {
	if enabled {
		return visionCheckBlock
	}
	return ""
}
