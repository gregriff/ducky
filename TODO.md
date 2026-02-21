## TODO List

#### High Priority
- sql from SQL_BUG.txt, if pasted into prompt, freezes entire program

#### 100 Go Mistakes Lessons:
- use variadic options to init TUIModel from CLI args

#### Bugs:
- cursor is broken since migration to bubbletea V2 (tell it to blink, seperate from the focus cmd now)
- cursor should be placed at end of line when placeholder shows up
- textarea is not foused on startup on tmux
- scrollback history is not preserved after clearing history. Fix this by adding an `entireHistory` field to chat.Model, where `history` is initially this same slice, but after a CLEAR HISTORY, it becomes a re-slice of `entireHistory`. only the scrollback functionality should use `entireHistory`. refer to book page 67 when implementing this

#### UI:
- insert 1 newline of top padding when rendering reasoning text
- move horizontal padding out into the view functions. dont pad in md renderer. add left gutter for copy?
- add popup command menu when holding ctrl
- insert newline into textarea once V2 is used
- add prompt editor (double clicking prompt), invoke $EDITOR on prompt buf. would be useful for adding code comment context after a paste
- scrollback should be able to use previous prompts after history is cleared. will need a seperate copy of the history for this, that is never reset.
- mark prompt lines in new selection gutter on the left side of screen
- impl discoloring/stop blinking when focus is lost
- impl some consistent scrolling or positioning when user clicks enter to submit a prompt
- add messages for history cleared etc.
- File uploads by drag/dropping into terminal

#### Rendering:
- Hyperlinks/citations, at least for claude models, as terminal hyperlinks: https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
- Fix Markdown H2s

#### Model Support:
- impl usage cost caluclation
- fully impl ability to switch models mid-session with a command, keeping history. use a selector bubble
- use contexts with streaming to cancel after 10 secs of no API response, resetting this timer if a chunk is recieved
- modify system prompt for current chat in TUI (popup bubble)

#### Configuration:
- add color configs:
  - UI elements


#### Selection/Copying:
##### Problem:
tmux offers the `copy-mode` command (`LEADER + [`), which takes control away from the application to allow selecting and copying. I could invoke this command instead of `less` on double-click, but copying more than what is currently on-screen would not be possible because tmux would scroll back in the history, above the application. This is also a problem with `less`, as terminal/tmux is still handling mouse events, so scrolling up while copying goes up in the terminal history as well.

##### Solution:
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

#### DX:
- refactor subcomponents to adhere to the ELM framework
- add pre-commit hooks
  - secrets
  - formatter
  - auditor?
  - one that checks for log.Print*

###### Profiles:
> If glamour adds support for specifying a lexer
Preset config options to use for the chat
- users can say "use the python profile" and then we just specify the Python Chroma lexer instead of having the lexer slowly guess from the chat history
- users can specify multiple (programming language) profiles to use more lexers
- profiles can change system messages of course
> Examples: `cli profile create (or create-profile?)` `cli run --profile=` `cli profile edit (or edit-profile?)` `cli profile set-default (or set-default-profile?)`

#### Random
###### Sliding-windowesque rendering of chat history:
Only render last N prompts/responses depending on their length, because the user won't resonably scroll up to view those. would probably require bubblezone-like marking of the renderedHistory stringbuilder during writing it, with invisible ANSI codes. or could ditch the stringbuilder and lazy-render the markdown from the stored rawtext. This would be complicated so probably don't do this but it would reduce CPU usage during window resizing. prob not worth it
- profile renderCurrentResponse(), prealloc space for responses using maxTokens

#### Testing:
- Ubuntu, fedora
- iTerm2, kitty, Konsole, Apple
- Zellij
