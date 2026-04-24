# TODO
## Phase One
- [x] Add a loader when you start a up so that you can figure out if the person is logged in or not
- [x] Add a new papermap logo and edit first screen view with it
- [x] Figure out how to prevent scrolling when you're in the papermap application
- [x] Figure out how to place the question prompt at the bottom of the screen
- [x] Add a dialog box when you quit for the user to confirm that they want to quit
- [x] Figure out how to have it such that the answers show up as scrollabele text
- [x] Figure out how to display different tables and types like that when the results are loaded from the API
- [x] Build the switching between workspaces UI
- [x] test it on other terminals
- [x] Figure out a means of properly storing the credentials from the users
- [x] Figure out how to not be able to revert the binary file
- [x] Add a better means of authenticating the user in the auth backend
- [x] Figure out how to fix scrolling with the mouse

## Phase Two
- [x] Figure out how to display the content as it streams, particularly the thinking process.
- [ ] Add rendering of the different types of charts
- [ ] Figure out how to add new workspaces from the UI
- [ ] Figure out how to run shell commands in the chat
- [ ] think about the ability to prepend some things like an md file to the prompts that you send to Alan

## Code Quality Followups
- [x] Wire `context.Context` cancellation through SSE/HTTP insight goroutines so quit / Clear / workspace switch tear down cleanly (today they use `context.Background()`).
- [x] Carry session-expiry detection through `insightHTTPResultMsg` (currently dropped in `app.go`'s HTTP-only error path, so users can stay signed-in with stale creds).
- [x] Extract `Model.Update` (cyclomatic 53) into per-message handlers (`handleStartup`, `handleWorkspacesLoaded`, `handleInsight*`, `handleSessionExpired`, `handleKeyPress`).
- [x] `GenerateRequestID` collisions: replace `time.Now()` modulo trick with `crypto/rand` or UUID.
- [x] `startInsightWithRetry` uses `time.Sleep` ignoring ctx — switch to `time.NewTimer` + `select`.
- [x] `sessionExpiredFromError` falls back to substring matching because some `fmt.Errorf("%v", err)` breaks `errors.Is`/`Unwrap`. Find the offending `%v` and switch to `%w`.
- [x] Centralise duplicated overlay-centering logic (`overlayQuitDialog` vs `overlayWorkspacePicker`).
- [x] Decide on dead `InsightResponse`/`InsightRequest` fields: trim or comment as forward-compat.
- [x] Move repeated hex colors (`#2ED8A3`, etc.) into `internal/theme`.
