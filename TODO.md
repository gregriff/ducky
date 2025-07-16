## TODO List
- rename project

#### UI:
- fix scrolling of textarea (use bubblezone to multiplex)
- move horizontal padding out into the view functions. dont pad in md renderer. add left gutter for copy?
- when textarea empty, keypad up/vim up cycles up in history. when at last char in textarea, keypad down/vim down cycles down in history if any
- impl discoloring/stop blinking when focus is lost
- impl some consistent scrolling or positioning when user clicks enter to submit a prompt
- add popup command menu when holding ctrl
- File uploads by drag/dropping into terminal

#### Rendering:
- Hyperlinks/citations, at least for claude models, as terminal hyperlinks: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
- Fix Markdown H2s
- make custom glamour stylesheet to render the centered H2s

#### Model Support:
- fully impl ability to switch models mid-session with a command, keeping history. use a selector bubble
- use contexts with streaming to cancel after 10 secs of no API response, resetting this timer if a chunk is recieved
- impl openAI models

#### Configuration:
- use XDG_CONFIG
- complete viper config file stuff
- add more config options
- add YAML file spec and put it at top of a demo-config.yaml in the repo
- add color configs:
  - glamour md stylesheet or at least style preset
  - pager prompt
  - UI elements

#### Profiles:
Preset config options to use for the chat
- users can say "use the python profile" and then we just specify the Python Chroma lexer instead of having the lexer slowly guess from the chat history
- users can specify multiple (programming language) profiles to use more lexers
- profiles can change system messages of course
> Examples: `cli profile create (or create-profile?)` `cli run --profile=` `cli profile edit (or edit-profile?)` `cli profile set-default (or set-default-profile?)`

#### Pager
##### General:
- option to load pager into a file with only the current prompt/response, use the less commands :n,:p to navigate through responses:
  - :n,:p would load pager at top of file (prompt)
> this would make saving response context to Marks easier but not nessicary
- better calculate line number to open pager in by taking into account HeaderView.Height and mouseMsg.Y. In order to try to get the pager to open the text in the exact same position as it is on screen

##### Marks:
- figure out showing them in pager without truncated line symbol (add H_PADDING in View function instead of in RenderHistory so text is always less wide than text+Mark status bar width)
- save them to DB


#### Selection/Copying:
##### Problem:
tmux offers the `copy-mode` command (`LEADER + [`), which takes control away from the application to allow selecting and copying. I could invoke this command instead of `less` on double-click, but copying more than what is currently on-screen would not be possible because tmux would scroll back in the history, above the application. This is also a problem with `less`, as terminal/tmux is still handling mouse events, so scrolling up while copying goes up in the terminal history as well.

##### Solution 1:
Use $EDITOR instead of pager and assume people know how to copy text in their editor and up to them to enable syntax highlighting for it
##### Solution 2:
manually impl text selection and copying:

Vars:
- `vpY = msg.Y - headerView.Height`
- `trueLineNo = vpY + vp.YOffset`

Impl:
- use `trueLineNo` to record start, end Line Number of selection mouse drag
  - if `vpY <= 0` invoke scrollUp by `vp.MouseDelta`
- denote selected lines with a 1char-wide left sidebar
- send start, end lines (ordered from least to greatest) to ChatHistory to getSelectedLines
- using ChatModel.history and associated styles.constants.V_PADDING to know which prompts/responses are being selected and exactly which lines and grab the selected text
- remove escape codes and add to clipboard

Notes:
- bubbletea v2 will add control for mouse cursor to make this look nicer
- double click a code block to copy the entire block

##### Solution 3:
Do both, opening editor on doubleclick and dragging does native select

#### DX:
- bubbletea v2
- add pre-commit hooks
  - secrets
  - formatter
  - auditor?
  - one that checks for log.Print*

#### Testing:
- Ubuntu, fedora
- iTerm2, kitty, Konsole, Apple
- Zellij
