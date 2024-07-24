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
* Un-Acknowledge incidents (re-assign to the Escalation Policy)

Planned Features:

* View arbitrary incidents
* Assign incidents to any PagerDuty User ID
* Edit incident titles
* Merge incidents

## Configuration

SREPD uses the [Viper](https://github.com/spf13/viper) configuration setup, and will read required values from `~/.config/srepd/srepd.yaml`.

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
* terminal: Your choice of terminal to use when launching external commands. Defaults to `/usr/bin/gnome-terminal`.

__NOTE:__ The cluster_login_cmd and terminal accept a variable for `%%CLUSTER_ID%%` to stand in for the Cluster ID in the command. At least one of the two, most likely `cluster_login_cmd` MUST have the `%%CLUSTER_ID%%` placeholder set. See [AUTOMATIC LOGIN FEATURES](#automatic-login-features) for more details about config variables.

An example srepd.yaml file might look like so:

```
---
# Editor will always be overridden by the ENV variable
# unless the ENV is not set for some reason
editor: vim

# Cluster Login options
# Note the trailing `--` is necessary for gnome-terminal and may be necessary
# for other terminals as well
terminal: /usr/bin/gnome-terminal --
cluster_login_cmd: ocm-container --clusterid %%CLUSTER_ID%%

# Note that aliases, etc, are not sourced by the shell command when launching.
# This means, for example, that `ocm-container`, as normally setup using an
# alias, does not work, but calling the command or a symlink directly does.

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
  - <pagerDuty User ID>
  - <pagerDuty User ID>
```

## Try it out

The easiest way to get started with SREPD, after adding the required config file, is to just clone this repository and run `go build -o ${GOPATH}/bin/srepd .`

## Automatic Login Configuration

To enable automatic login directly from SREPD you will need to configure the `terminal` and `cluster_login_command` settings in the srepd config file.

### Linux
A typical linux configuration to launch a new terminal window may look something like what follows. Be sure to change to your preferred terminal or preferred ops environment (ocm-container, ocm-backplane session, osdctl session, etc)

```yaml
# Gnome-Terminal
# gnome-terminal requires the "--" separator between the terminal any any of its flags and the command to be run
# eg: gnome-terminal -- ocm backplane login %%CLUSTER_ID%%
terminal: gnome-terminal --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

Each terminal has its own way of accepting an argument for a command to run after launching. Some examples known to work:


```yaml
# Contour (in this example, via Flatpak)
# contour does not require any special arguments or separators
terminal: flatpak run org.contourterminal.Contour
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# BlackBox (via Flatpak)
terminal: flatpak run com.raggesilver.BlackBox --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

```yaml
# Terminator
# terminator requires the "-x" or "--execute" flag as the separator between terminal arguments and the command to be run
terminal: terminator --execute
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```


### MacOS
Configuration for MacOS is not as straightforward because of the way that MacOS handles applications differently from Linux. We've provided a few separate ways to configure SREPD to handle automatic logins.

#### Default MacOS terminal
The following configuration will launch a new MacOS default terminal window to automatically login to a cluster. This example uses ocm-container but feel free to modify to fit your preferred workflow.

```yaml
# MacOS Terminal
terminal: osascript -e
cluster_login_command: tell application "Terminal" to do script "ocm-container -C %%CLUSTER_ID%%"
```

#### iTerm2 Support
iTerm2 is even more special in the fact that you can't tell it to 'do script' like you can with the default Terminal on MacOS. You will need to create a separate shell script to invoke from SREPD that will then run the osascript command needed to create a new iTerm2 window/tab and call the login script.

```bash
#!/bin/bash

osascript \
  -e 'tell application "iTerm" to tell current window to set newWindow to (create tab with default profile)' \
  -e "tell application \"iTerm\" to tell current session of newWindow to write text \"${*}\""
```

Then, you would add the following to your srepd config:

```yaml
# iTerm2
terminal: /Users/kbater/Projects/spikes/srepd
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

### TMUX Support

Alternatively to launching a whole new terminal window, if you're already using tmux you can use the following configuration to create new tmux-windows and auto-launch your environment from there:

```yaml
# TMUX
terminal: tmux new-window --
cluster_login_command: ocm backplane login %%CLUSTER_ID%%
```

<a name="automatic-login-features"></a>
### Automatic Login Features
The first feature of Automatic Login is the ability to replace certain strings with their cluster-specific details. When you pass `%%VARIABLE%%` in your `terminal` or `cluster_login_command` configuration strings they will dynamically be replaced with the alert-specific variable. This allows you to be able to put the specific details of these variables inside the command. The first argument of the `terminal` setting MUST NOT BE a replaceable value.

Currently, only `%%CLUSTER_ID%%` is supported. It's also important to note that if `%%CLUSTER_ID%%` does NOT appear in the `cluster_login_command` config setting that the cluster ID will be appended to the end of the cluster login command. If the replacable `%%CLUSTER_ID%%` string is present in the `cluster_login_command` setting, it will NOT be appended to the end.

Examples:
```
## Assume the cluster ID for these examples is `abcdefg`

cluster_login_command: ocm backplane login %%CLUSTER_ID%% --multi
## effectively runs "ocm backplane login abcdefg --multi"

cluster_login_command: ocm backplane login --multi
## effectively runs "ocm backplane login --multi abcdefg"
```
