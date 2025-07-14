## TODO List
- rename project

#### UI:
- add thinking, first response blinkers
- shrink textarea while streaming (no input allowed)
- impl some consistent scrolling or positioning when user clicks enter to submit a prompt
- fix scrolling of textarea (use bubblezone to multiplex)
- File uploads by drag/dropping into terminal

#### Rendering:
- fix debounced markdown renderer resizing
- Hyperlinks/citations, at least for claude models, as terminal hyperlinks: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
- Fix Markdown H2s
- make custom glamour stylesheet to render the centered H2s

#### Model Support:
- fully impl ability to switch models mid-session with a command, keeping history
- use contexts with streaming to cancel after 10 secs of no API response, resetting this timer if a chunk is recieved
- impl openAI models

#### Configuration:
- complete viper config file stuff
- add more config options
- add YAML file spec and put it at top of a demo-config.yaml in the repo
- add color configs:
  - glamour stylesheet
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
