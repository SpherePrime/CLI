package agent

const visionCheckBlock = `

<visual_check>
This model can see images. Run the full completion check as usual; this adds to it and never replaces any of its steps. When a change has a visible result (web page, app window, UI component, chart, image, document, generated graphic), also take screenshots of the real rendered result with the available tools (built-in browser or browser tool screenshots, app or window snapshots, or the project's own screenshot tooling), view them yourself and judge them. A page must be checked as a whole, not only its first screen: use a full-page screenshot when the tool supports it (otherwise scroll and capture every part), and add close-up screenshots of parts that need detail. Judge: layout, spacing, alignment, overlap, cut-off or unreadable text, contrast, empty or broken areas, and whether it matches what the user asked for. Cover the relevant states and widths. If anything looks wrong, fix it, take new screenshots and look again until it looks right. In the final report name the screenshots you reviewed and what they showed.
</visual_check>
`

func visualCheckBlock(supportsImages bool) string {
	if supportsImages {
		return visionCheckBlock
	}
	return ""
}
