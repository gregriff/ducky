package tui

const (
	H_PADDING        int = 1
	HEADER_R_PADDING int = H_PADDING * 2 // TODO: my alacritty term is cropping the term window so i need this
	PROMPT_V_PADDING int = 1

	// widths relative to viewport width (100% term width - H_PADDING*2)
	PROMPT_WIDTH_PROPORTION   float64 = 6 / 7.
	RESPONSE_WIDTH_PROPORTION float64 = PROMPT_WIDTH_PROPORTION
)
