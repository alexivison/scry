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

## Commit integration

With `--commit`, you can generate and execute commits without leaving the tmux pane:

```bash
tmux split-window -h "cd /path/to/repo && scry --base origin/main --head HEAD --watch --watch-interval 2s --commit"
```

Press `c` in the file list to generate a commit message, `e` to edit, `Enter` to confirm, or `Esc` to cancel.

## Notes

- The `--watch` flag and idle screen are fully supported. When no files have diverged, Scry shows an idle screen that auto-transitions to the file list on first detected change.
- The `--commit` flag requires an `ANTHROPIC_API_KEY` environment variable.
- Scry recovers cleanly from terminal resize events within the tmux pane.
