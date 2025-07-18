# gpt-cli-go

Gpt-cli-go aims to provide the best LLM chat interface for developers. With a minimal UX and a UI inspired by text messaging apps, gpt-cli-go lets you access the most powerful LLM's within the terminal via an intuitive interface.

### Installation

`go install github.com/gregriff/gpt-cli-go`

### Configuration
Copy `gpt-cli-go.toml` into `$XDG_CONFIG_HOME/gpt-cli-go/gpt-cli-go.toml` and insert your configuration

### Q&A
- *Why the terminal?*
> This is for developers. I like to juggle several chats at once, and I'd rather let tmux handle that instead of having several LLM browser tabs open.

- *Does this support agentic workflows?*
> No. I'll let the IDEs and model providers handle that. This is a simple prompt/response loop.

- *Why not use the IDE's LLM interface?*
> I believe LLMs work best in a seperate window when writing software. This makes the programmer think more about their prompts and discourages vibe coding.

### Customizing

Glamour is the package responsible for rendering Markdown. It can be configured with different [styles](https://github.com/charmbracelet/glamour/tree/master/styles), which are JSON files that define mappings of strings to Markdown tokens, as well as colors, spacing options, and "code themes", which determine how code-block Markdown is rendered. These code themes can be changed with the `code_block.theme` property and can include any theme from [this list](https://github.com/alecthomas/chroma/tree/master/styles)

To create your own style, copy a file from [glamour's built-in styles](https://github.com/charmbracelet/glamour/tree/master/styles) and modify it to change colors or other properties. Of note is the top-level `document.block_prefix` property, which if not set to "", will result in spacing inconsistencies when a response stream completes.. Then, place the JSON file in `$XDG_CONFIG_HOME/gpt-cli-go/styles`, and refer to it in the CLI args or the `.toml` config file by absolute path.

### Roadmap
[Can be found here](./TODO.md)

### Supported Platforms

Development is done on macOS, using alacritty and tmux, or Zed's terminal. Testing will be done on fedora and raspberry pi OS soon.

### Additional Info

This project is a rewrite of the now-unmaintained GPT-CLI Python repository. I'm doing this rewrite to learn Go and get experience with its concurrency patterns, standard library, package ecosystem, and idioms.
