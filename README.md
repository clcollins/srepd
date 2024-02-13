# SREPD

A PagerDuty terminal user interface focused on common SRE tasks

**Note: This project is still in Alpha phase and there are bugs to be squashed.**

Features:

* Retrieve and list incidents assigned to the current user, and their team(s)
* Vew a summary of an incident, including alerts and notes
* Add a note to an incident
* Reassign incidents to a (configured) "silent" user (ie. silence the alert)
* Acknowledge incidents
* Resizes nicely(-ish) when the terminal is resized

Planned Features:

* Un-Acknowledge incidents (re-assign to the Escalation Policy)
* View arbitrary incidents
* Assign incidents to any PagerDuty User ID
* Edit incident titles
* Merge incidents

## Configuration

SREPD used the [Viper](https://github.com/spf13/viper) configuration setup, and will read required values from `~/.config/srepd/srepd.yaml`.

Configuration variables have the following precedence: 

`command line arguments > environment variables > config file values`

**Required Values**

* token: A PagerDuty Oauth Token
* teams: A list of PagerDuty team IDs to gather incidents for
* silentuser: A PagerDuty user ID to receive "silenced" alerts (or to troll, you do you)

**Optional Values**

* ignoredusers: A list of PagerDuty user IDs to exclude from retrieved incident lists.  It's recommended that the "silentuser" ID is in this list.
* editor: Your choice of editor.  Defaults to the `$EDITOR` environment variable.
* cluster_login_cmd: Command used to login to a cluster from SREPD.  Defaults to `/usr/local/bin/ocm backplane login`
* shell: Your choice of shell to use inside of launched terminal windows. Defaults to `$SHELL`.
* terminal: Your choice of terminal to use when launching external commands. Defaults to `/usr/bin/gnome-terminal`.

An example srepd.yaml file might look like so:

```
---
# Editor will always be overridden by the ENV variable
# unless the ENV is not set for some reason
editor: vim

# Cluster Login options
cluster_login_cmd: "ocm-container"
shell: /bin/bash
terminal: /usr/bin/gnome-terminal

# Note that aliases, etc, are not sourced by the shell command when launching.
# This means, for example, that `ocm-container`, as normally setup using an alias, does not work, but calling the command or a symlink directly does.

# More complicated commands can be specified with space-separated strings
# terminal: "flatpak run org.contourterminal.Contour"

# PagerDutyOauthToken
token: <pagerDuty Oauth Token>

# Teams are PagerDuty team IDs to retrieve incidents
teams:
  - <pagerDuty Team ID>

# Silent User is a PagerDuty User ID to assign issues to "silence" them
silentuser: <pagerDuty User ID>

# Ignore Users is a list of PagerDuty User IDs to ignore when gathering incidents
ignoredusers:
  - <pageDuty User ID>
  - <pagerDuty User ID>
```

## Try it out

The easiest way to get started with SREPD, after adding the required config file, is to just clone this repository and run `go build -o ${GOPATH}/bin/srepd .`
