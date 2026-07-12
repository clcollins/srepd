# Quickstart

## Key Bindings

| Key | Action |
|-----|--------|
| h | help |
| ↑/k | up |
| ↓/j | down |
| g | jump to top |
| G | jump to bottom |
| enter | view |
| esc | back |
| ctrl+q/ctrl+c | quit |
| t | toggle team/individual |
| r | refresh |
| ctrl+r | toggle auto-refresh |
| n | add note |
| ctrl+s | silence |
| a | acknowledge |
| ctrl+e | re-escalate |
| ctrl+a | toggle auto-acknowledge |
| u | toggle urgency filter |
| :/ | command input |
| l | login to cluster |
| o | open in browser |
| s | open SOP |
| ctrl+l | view debug log |
| m | merge incident |
| w | toggle watcher |
| ctrl+t | add tags |
| tab/→ | next tab |
| shift+tab/← | prev tab |
| ctrl+h | docs |

## Chord Commands (ctrl+x + key)

| Key | Action |
|-----|--------|
| ? | show chord help |
| b | rosa-boundary login |
| d | view debug log |

## Input Commands

| Command | Action |
|---------|--------|
| :agent <query> | ask Claude AI |
| :watcher <query> | query AI watcher |
| :flag cluster <id> | flag incidents by cluster ID |
| :flag org <pattern> | flag incidents by org name |
| :unflag <id> | remove a flag condition by ID |
| :unflag all | clear all flag conditions |
| :flags | list all flag conditions |
| :flags save [path] | save flags to file |
| :flags load [path] | load flags from file |
