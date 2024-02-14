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

SREPD uses the [Viper](https://github.com/spf13/viper) configuration setup, and will read required values from `~/.config/srepd/srepd.yaml`.

**Required Values**

* token: A PagerDuty Oauth Token
* teams: A list of PagerDuty team IDs to gather incidents for
* silentuser: A PagerDuty user ID to receive "silenced" alerts (or to troll, you do you)

**Optional Values**

* ignoredusers: A list of PagerDuty user IDs to exclude from retrieved incident lists.  It's recommended that the "silentuser" ID is in this list.
* editor: Your choice of editor.  This will ALWAYS lose precedence to the `$EDITOR` environment variable, unless the ENV VAR is not set for some reason.

An example srepd.yaml file might look like so:

```
---
# Editor will always be overridden by the ENV variable
# unless the ENV is not set for some reason
editor: vim

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
