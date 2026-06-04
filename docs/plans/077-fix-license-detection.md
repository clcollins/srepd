# Fix GitHub License Detection

## Problem
GitHub's license detection (licensee) fails when non-standard text is
appended to the LICENSE file, causing the license badge to show unknown.

## Solution
Move AI contributions note from LICENSE to a dedicated README section.
LICENSE remains standard MIT template text only.
