package agent

const visionCheckBlock = `

<visual_check>
Skip <visual_check> entirely (no browser_run, no screenshots) for any part a review agent already APPROVED: an APPROVED part is already visually verified.
This model can see images. For visible results that no review agent approved, run the full completion check as usual; this adds to it and never replaces any of its steps.
When an unreviewed change has a visible result (web page, app window, UI component, chart, image, document, generated graphic), exercise it and look at it yourself:
- Interact, do not only look: click the changed controls, fill and submit forms, open and close menus and dialogs, hover and focus interactive elements, and trigger empty, loading and error states. In Prime Desktop's built-in browser write this as one browser_run scenario with screenshots at the moments that matter; in CLI mode use our browser MCP (browser_run) when available, or our prime-designer skill (scripts/audit.mjs) and Playwright / Chrome to capture screenshots and inspect them with the view tool.
- Prefer the browser: when an app's interface is a web frontend (Vite, Next, Tauri or Electron renderer and similar), start its dev server with the backend in the background and check the UI in the browser (built-in browser in Desktop, or our browser MCP / Playwright / Chrome via our prime-designer skill in CLI) instead of launching the full native app and capturing its window. Use app or window tools only for what the browser cannot show (native windows, menus, tray, installers).
- Overview first, then close-ups: a full-page or whole-window image reaches the model heavily downscaled, so judge only composition from it. Check details on close-ups: screenshots of single components with padding, zoom 2 for small text, icons, borders and focus rings, clip regions for places where blocks meet, and tiles for long pages.
- Judge layout, spacing, alignment, overlap, cut-off or unreadable text, contrast, empty or broken areas, and whether it matches what the user asked for, at the relevant widths and states.
If anything looks wrong, fix it, take new screenshots and look again until it looks right. In the final report name the screenshots you reviewed and what they showed, or cite the review agent's APPROVED screenshots and checks instead of re-shooting.
</visual_check>
`

func visualCheckBlock(enabled bool) string {
	if enabled {
		return visionCheckBlock
	}
	return ""
}
