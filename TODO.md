## Translation Todos:
- n/A

## Additional Features:
- finalize prompt and response margins, both of which are broken. ensure floats are used in intermediate width calcs and the final col count is then cast to int
- impl select+copying of viewport content
- fix scrolling of textarea
- impl some consistent scrolling or positioning when user clicks enter to submit a prompt
- shrink textarea while streaming (no input allowed)
- use bubblezones to multiplex scrolling between viewport and textarea?
- debounce scroll/slow it
- use contexts with streaming to cancel after 10 secs of no API response, resetting this timer if a chunk is recieved
- fix debounced markdown renderer resizing
- add thinking, first response blinkers
- add textbox component
- make custom glamour stylesheet to render the centered H2s
- move commands section
- fully impl ability to switch models mid-session with a command, keeping history
- impl openAI models
- File uploads by drag/dropping into terminal
- Rendering of hyperlinks/citations, at least for claude models
- Add precommit hooks
