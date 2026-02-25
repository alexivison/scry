# tmux Session Integration

Scry can run as a long-lived process in a tmux pane, providing a persistent diff view alongside your editor and other tools.

## Launch pattern

```bash
# In your tmux session setup script:
tmux split-window -h "cd /path/to/repo && scry --base origin/main --head HEAD --watch --watch-interval 2s"
```

## Requirements

- Scry must remain attached to its pane and recover cleanly from pane resize.
- Exit is explicit: press `q` in the Scry pane or kill the pane.
- Scry has no dependency on other panes — it is independently restartable.

## Example session layout

```
+-------------------+------------------+
|                   |                  |
|  Editor / CLI     |  Scry (watch)    |
|                   |                  |
|                   |  auto-refreshes  |
|                   |  on changes      |
+-------------------+------------------+
```

## Notes

- The `--watch` flag and idle screen are planned for v0.2.
- In v0.1, you can launch Scry manually and press `r` to refresh after making changes.
