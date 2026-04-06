# Dev.to Comments for vault-sync

## Comment 1: On "Why I switched from iCloud to Git for my Obsidian vault"

Been there with the iCloud conflicts. We built [vault-sync](https://github.com/drolosoft/vault-sync) specifically because of this pain point. It's just git + Obsidian, nothing fancy — handles the sync across machines and you actually see what changed. No magic, no mysterious conflicts at 2am. If you want to avoid the manual git workflow setup, it's literally `git init` on steroids.

---

## Comment 2: On "The best dotfile managers in 2026"

Good list. One thing I'd add though — if you're already managing your Obsidian vault with git (which honestly you should be), [vault-sync](https://github.com/drolosoft/vault-sync) does that whole sync layer for you. Treats your vault as the source of truth and keeps it in sync across whatever machines you throw at it. Works alongside your dotfile manager, not against it.

---

## Comment 3: On "My Obsidian workflow for software engineers"

Your Syncthing setup is solid. We went the git route with [vault-sync](https://github.com/drolosoft/vault-sync) because it gives you the history and you're not hoping sync picked the right version when conflicts happen. Plus you can diff your notes like code. Different tradeoff — less real-time, more control. Either way, glad to see people solving this properly instead of just using Obsidian Sync.
