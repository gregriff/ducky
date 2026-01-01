module github.com/gregriff/ducky

go 1.25.1

require (
	charm.land/bubbles/v2 v2.0.0-rc.1
	charm.land/bubbletea/v2 v2.0.0-rc.2
	charm.land/glamour/v2 v2.0.0-20251110203732-69649f93d3b1
	charm.land/lipgloss/v2 v2.0.0-beta.3.0.20251114164805-d267651963ad
	github.com/anthropics/anthropic-sdk-go v1.19.0
	github.com/atotto/clipboard v0.1.4
	github.com/lrstanley/bubblezone/v2 v2.0.0-alpha.3
	github.com/muesli/reflow v0.3.0
	github.com/openai/openai-go/v3 v3.15.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/viper v1.21.0
	golang.org/x/term v0.32.0
)

require (
	github.com/alecthomas/chroma/v2 v2.20.0 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.3.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20251116181749-377898bcce38 // indirect
	github.com/charmbracelet/x/ansi v0.11.1 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250919153222-1038f7e6fef4 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.6.1 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yuin/goldmark v1.7.13 // indirect
	github.com/yuin/goldmark-emoji v1.0.6 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)

// custom packages
replace github.com/muesli/termenv => github.com/gregriff/termenv v0.16.200

//replace github.com/muesli/termenv => ../../oss/termenv
