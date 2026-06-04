# OCM Auth Browser Notification

## Problem
When srepd starts with expired OCM tokens, InitiateAuthCode() opens a
browser for OAuth login with no user-visible message. The app appears
to hang.

## Solution
Add stderr messages before and after InitiateAuthCode() in NewClient().
This runs only on startup before the TUI starts.
