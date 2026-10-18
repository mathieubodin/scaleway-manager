---
name: mathieubodin-sdlc-setup
description: Installs or updates the mathieubodin-sdlc module (skills sdlc-work and sdlc-gate-page, bmad-help rows). Use when the user asks to install, update or reinstall the SDLC module.
---

# Install the SDLC module

The BMad installer registers this module: it copies the skills to `.claude/skills/` and merges `assets/module-help.csv` into `_bmad/_config/bmad-help.csv`. This skill runs no script of its own.

From the project root, run:

```bash
npx bmad-method@6.12.4 install --directory "$PWD" --custom-source "$PWD" --tools claude-code --yes
```

Always pass `--custom-source`: on BMad 6.12.4, an update without it fails for local custom modules (BMAD-METHOD issue #2869).

Never edit `_bmad/_config/bmad-help.csv` by hand. After a change under `skills/`, run the command again and commit the files it changed.
