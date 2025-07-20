# ducky

![preview](./docs/overview.gif)

Ducky provides the most ergonomic LLM chat interface for developers. With a minimal UX inspired by text messaging apps, ducky lets you access most powerful LLMs from within the terminal.

### Installation
1. Install Go
2. `go install github.com/gregriff/ducky@latest`

### Usage
`ducky run [model name]`
> Currently only Anthropic models are supported. OpenAI coming soon

> Run `ducky --help` to see all flags and options

### Configuration
Edit the `$XDG_CONFIG_HOME/ducky/ducky.toml` that was created for you.

### Features
- Markdown rendering of responses (can customize colors and more)
- Syntax highlighting of code blocks (configurable, and per-language highlighting coming soon)
- Double-click a response to open the `less` pager for easier copying and incremental text search
- Responsive resizing of all elements on screen during terminal window resizing, even during response streaming
- Intelligent resizing of prompt text input to maximize main content area
- Graceful handling of API errors

### Q&A
- *Why the terminal?*
> I like to juggle several chats at once, and I'd rather let tmux handle that instead of having several LLM browser tabs open. Also, all IDEs have a terminal, so any developer can easily incorporate this tool into their existing workflow.

- *Does this support agentic workflows?*
> No. I'll let the IDEs and model providers handle that. This is a simple prompt/response loop.

- *Why not use the IDE's LLM interface?*
> I believe LLMs work best in a seperate window when writing software. This makes the programmer think more about their prompts and discourages vibe coding.

- *Why include the `less` pager?*
> I thought it may be useful to allow full-text search of the chat history. Instead of implementing this from scratch I decided to use a familiar tool that was made specifically for this use case.

- *How to select text and copy to clipboard?*
> tldr; For now, you are only able to select and copy text that is visible on screen, using either the terminal emulator or multiplexer's dedicated text-selection keybind.

> Unfourtunately, this is made tricky by the nature of the terminal. Since ducky is a uses the terminal's fullscreen mode for a polished feel, it captures all mouse/scroll input from it. While terminals usually have a keybind for overriding this (on alacritty its `shift+left click` to begin highlighting text, on others its `fn+left click`), these solutions only allow copying text visible on the screen. Tmux provides the same behavior with its `copy-mode` by pressing `leader+[`. So, if while selecting text with either of these methods, you drag the mouse to the top of the screen in order to scroll the text up and continue copying it, the screen will not scroll up, because the mouse and scroll inputs are being temporarily handled by the terminal emulator or multiplexer. A solution to this is for me to implement a text selection/copying feature myself, which is planned.

### Customizing

Glamour is the package responsible for rendering Markdown. It can be configured with different [styles](https://github.com/charmbracelet/glamour/tree/master/styles), which are JSON files that define mappings of strings to Markdown tokens, as well as colors, spacing options, and "code themes", which determine how code-block Markdown is rendered. These code themes can be changed with the `code_block.theme` property and can include any theme from [this list](https://github.com/alecthomas/chroma/tree/master/styles)

To create your own style, copy a file from [glamour's built-in styles](https://github.com/charmbracelet/glamour/tree/master/styles) and modify it to change colors or other properties. Of note is the top-level `document.block_prefix` property, which if not set to "", will result in spacing inconsistencies when a response stream completes. Then, place the JSON file in `$XDG_CONFIG_HOME/ducky/styles`, and refer to it in the CLI args or the `.toml` config file by absolute path.

### Roadmap
[Can be found here](./TODO.md)

### Supported Platforms

Development is done on macOS, using alacritty and tmux, or Zed's terminal. Testing will be done on fedora and raspberry pi OS soon.

### Additional Info

This project is a rewrite of the now-unmaintained GPT-CLI Python repository. I'm doing this rewrite to learn Go and get experience with its concurrency patterns, standard library, package ecosystem, and idioms.
