package agent

const visionCheckBlock = `

<visual_check>
Skip <visual_check> entirely (no browser_run, no screenshots) for any part a review agent already APPROVED: an APPROVED part is already visually verified.
This model can see images. For visible results that no review agent approved, run the full completion check as usual; this adds to it and never replaces any of its steps.
When an unreviewed change affects a UI or visible output (web page, app window, UI component, chart, image, document, generated graphic):
- Interact, do not only look: click the changed controls, fill and submit forms, open and close menus and dialogs, hover and focus interactive elements, and trigger empty, loading and error states. If only UI logic/behaviour changed (without layout or styling edits), interactive browser/app checks with state assertions and console/network inspection are required, while screenshots are optional.
- Prefer the browser: when an app's interface is a web frontend (Vite, Next, Tauri or Electron renderer and similar), start its dev server with the backend in the background and check the UI in the browser (built-in browser in Desktop, or our browser MCP / Playwright / Chrome via our prime-designer skill in CLI) instead of launching the full native app and capturing its window. Use app or window tools only for what the browser cannot show (native windows, menus, tray, installers).
- Match target widths to the product: for desktop/PC-only software, check normal desktop and resizable window sizes — do not test phone widths (320-375 px). Test mobile widths only for responsive websites, landings, or when phone support is requested.
- Overview first, then close-ups (when visuals or layout changed): a full-page or whole-window image reaches the model heavily downscaled, so judge composition from it. Take targeted close-ups (selector with padding, zoom 2, clip, or tiles for long pages) only where small text, icons, borders, focus rings, or suspected visual issues need closer inspection — do not spam redundant close-ups of unchanged areas.
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
