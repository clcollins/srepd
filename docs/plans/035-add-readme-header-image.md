# Plan 035: Add Header Image to README

## Status: Complete

## Objective

Add a header image to the top of the README to give the project a visual
identity and make the GitHub repository page more inviting.

## Changes

1. Add `img/srepd.jpg` -- pixel-art header image depicting an SRE at work
   with srepd on screen, taming a PagerDuty alert monster
2. Update `README.md` to display the image above the `# SREPD` heading

## Notes

- Image is stored in `img/` directory at the repository root
- Uses standard Markdown image syntax: `![srepd](img/srepd.jpg)`
- No alt-text beyond the project name since the image is decorative
