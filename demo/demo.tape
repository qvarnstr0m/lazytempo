# lazytempo demo tape (VHS — https://github.com/charmbracelet/vhs)
#
# Record with:  ./demo/record.sh    (builds the binary, starts the mock, runs this)
#
# Run vhs from the repo root. Assumes:
#   - ./lazytempo exists (a built binary)
#   - the mock API from ./demo is listening on :8787
# The app is pointed at the vanity host http://hollywood-upstairs.atlassian.net and reaches the
# mock via HTTP_PROXY, so the recording shows a realistic Jira site, fake data,
# and no local paths.

Output "assets/demo.gif"

Set Shell "bash"
Set FontSize 16
Set Width 1300
Set Height 720
Set Padding 18
Set TypingSpeed 60ms

# --- setup (hidden): isolate config/cache, launch cleanly as `lazytempo` ---
Hide
Type "rm -rf /tmp/lt-demo && mkdir -p /tmp/lt-demo/config /tmp/lt-demo/cache" Enter
Type "cp ./lazytempo /tmp/lt-demo/lazytempo" Enter
Type "export PATH=/tmp/lt-demo:$PATH && cd /tmp/lt-demo" Enter
Type "export XDG_CONFIG_HOME=/tmp/lt-demo/config XDG_CACHE_HOME=/tmp/lt-demo/cache" Enter
Type "export LAZYTEMPO_JIRA_BASE_URL=http://hollywood-upstairs.atlassian.net LAZYTEMPO_TEMPO_BASE_URL=http://hollywood-upstairs.atlassian.net" Enter
Type "export LAZYTEMPO_JIRA_EMAIL=nick.riviera@hollywood-upstairs.example LAZYTEMPO_JIRA_API_TOKEN=demo LAZYTEMPO_TEMPO_API_TOKEN=demo" Enter
Type "export HTTP_PROXY=http://localhost:8787 http_proxy=http://localhost:8787" Enter
Type "clear" Enter
Show

Type "lazytempo"
Enter
Sleep 2.8s

# browse issues (the Details panel updates)
Type "j"
Sleep 650ms
Type "j"
Sleep 650ms
Type "j"
Sleep 650ms
Type "k"
Sleep 700ms

# glance at the week panel, then back to the issues
Type "3"
Sleep 1.3s
Type "1"
Sleep 700ms
Type "k"
Sleep 700ms

# add a worklog on the selected issue (account preselected, ~1h, today)
Type "a"
Sleep 1.3s
Enter
Sleep 450ms
Enter
Sleep 450ms
Down 11
Sleep 600ms
Enter
Sleep 450ms
Enter
Sleep 450ms
Type "Reattach the other thumb"
Sleep 800ms
Enter
Sleep 2s

# show the help overlay, then close it
Type "?"
Sleep 2.4s
Escape
Sleep 1.2s
