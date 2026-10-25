# EzzyCraft mods

Put pack or item/block definition JSON files in this folder and commit them with the repo.

The server does not load every file automatically. After the game starts:

1. Open **Options...**
2. Unlock server settings with the operator code
3. Type the filename you committed, such as `cool-swords.json` or `cool-swords`
4. Choose **Load**

**Remove** in the manager unloads that mod for everyone. It does not delete the file from this folder.

Valid filenames use letters, numbers, dots, hyphens, and underscores, and must end in `.json`.

Pack files use schema `coop1.10.item-block-pack.v2`. Standalone item or block files use `coop1.10.item-block-definition.v1` or `.v2`. Export those from `Other/coop-sprite-studio.html`.
