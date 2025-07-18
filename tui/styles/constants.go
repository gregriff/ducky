package tui

const (
	H_PADDING        int = 1
	HEADER_R_PADDING int = H_PADDING * 2 // TODO: my alacritty term is cropping the term window so i need this
	PROMPT_V_PADDING int = 1

	// widths relative to viewport width (100% term width - H_PADDING*2)
	WIDTH_PROPORTION_PROMPT   float64 = 6 / 7.
	WIDTH_PROPORTION_RESPONSE float64 = 9 / 10.

	TEXTAREA_HEIGHT_COLLAPSED int = 1
	TEXTAREA_HEIGHT_NORMAL    int = 3

	// spacing between the main viewport and the textarea
	VP_TA_SPACING      string = "\n"
	VP_TA_SPACING_SIZE int    = len(VP_TA_SPACING)
)
