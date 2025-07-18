# gpt-cli-go

This project is a rewrite of the GPT-CLI Python repository. It aims to allow better configuration with [Viper](https://github.com/spf13/viper) and provide more features than the previous project (like automatic per-language syntax highlighting with [Chroma](https://github.com/alecthomas/chroma#identifying-the-language))

I'm doing this rewrite mainly to learn Go and get experience with its concurrency patterns, standard library and package ecosystem.


###  Customizing

Glamour is the package responsible for rendering Markdown. It can be configured with different "styles", which are JSON files that define mappings of strings to Markdown tokens, as well as colors, spacing options, and "code themes", which determine how code-block Markdown is rendered. These code themes can be changed with the `code_block.theme` property can include any theme from [this list](https://github.com/alecthomas/chroma/tree/master/styles)

To create your own style, copy a file from [glamour's built-in styles](https://github.com/charmbracelet/glamour/tree/master/styles) and modify it to change colors or other properties. Of note is the top-level `document.block_prefix` property, which if not set to "", will result in spacing inconsistencies when a response stream completes (because `tui/styles/constants.PROMPT_V_PADDING` != `document.block_prefix` by default). Then, place the JSON file in `$XDG_CONFIG_HOME/gpt-cli-go/styles`, and refer to it in the CLI args or the `.toml` config file by absolute path.
