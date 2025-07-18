## TODO List
- rename project

#### Bugs:
-

#### UI:
- during streaming, only render current prompt and response for better UX. upon stream completion, render entire history and reposition where user was at the moment the stream completed. if a scrollup happens at vp.YOffset==0, render entire history, reposition to scroll pos
- move horizontal padding out into the view functions. dont pad in md renderer. add left gutter for copy?
- when textarea empty, keypad up/vim up cycles up in history. when at last char in textarea, keypad down/vim down cycles down in history if any
- mark prompt lines in new selection gutter on the left side of screen
- impl discoloring/stop blinking when focus is lost
- impl some consistent scrolling or positioning when user clicks enter to submit a prompt
- add popup command menu when holding ctrl
- add messages for history cleared etc.
-
- insert newline into textarea once V2 is used
- File uploads by drag/dropping into terminal

#### Rendering:
- Hyperlinks/citations, at least for claude models, as terminal hyperlinks: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
- Fix Markdown H2s

#### Model Support:
- fully impl ability to switch models mid-session with a command, keeping history. use a selector bubble
- use contexts with streaming to cancel after 10 secs of no API response, resetting this timer if a chunk is recieved
- impl openAI models
- modify system prompt for current chat in TUI (popup bubble)

#### Configuration:
- add TOML file spec and put it at top of a demo-config.toml in the repo
- add color configs:
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

#### Random
###### Sliding-windowesque rendering of chat history:
Only render last N prompts/responses depending on their length, because the user won't resonably scroll up to view those. would probably require bubblezone-like marking of the renderedHistory stringbuilder during writing it, with invisible ANSI codes. or could ditch the stringbuilder and lazy-render the markdown from the stored rawtext. This would be complicated so probably don't do this but it would reduce CPU usage during window resizing. prob not worth it

#### Testing:
- Ubuntu, fedora
- iTerm2, kitty, Konsole, Apple
- Zellij
