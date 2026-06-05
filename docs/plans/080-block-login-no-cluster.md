# Block login when no cluster_id found

## Problem
Fleet-level alerts have no cluster_id. Pressing 'l' launched a
terminal with an empty cluster ID argument.

## Solution
Show a flash notification instead of launching the terminal.
