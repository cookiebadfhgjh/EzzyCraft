# EzzyCraft

## Starting the server

Build the server with `build.bat`, then start it with an operator code:

```powershell
.\mcserver.exe -op-code "your-private-code"
```

Open the server address in a browser. In game, open **Options...**, enter the operator code, and choose **Unlock Server Settings** to import item/block definition JSON files. Without `-op-code`, server settings and definition imports remain disabled.

The unlocked server settings also let an operator set the shared game mode, world time, and PvP. The first player to join is a chat operator; operators can promote other players with `/op <player>`.

## Operator chat commands

Type these in online chat (`T`). Only operators can change server settings or use administrative commands:

- `/help`, `/list`
- `/pvp [on|off]`, `/knockback <0-200>` (percentage; 0 disables knockback)
- `/gamemode <survival|creative>` (aliases: `/gms`, `/gmc`)
- `/time [query|set <HH:MM|hour|day|night|noon|midnight>]`
- `/give [player] <item name|id> [amount]` (amount 1–64; for example `/give @s diamond 5`)
- `/op <player>`, `/deop <player>`
- `/tp <player>` (teleports you to that player)

Shared definition JSON files are stored in `server_data\definitions` next to the server's working directory. The server creates this folder automatically and sends its definitions to connected players. Use HTTPS when exposing the server publicly so the operator code is protected in transit.

## Co-op Sprite Studio

Open `Other\coop-sprite-studio.html` to create custom items and blocks. The editor includes combat and mining stats, including a per-tool knockback multiplier, block drops and light, recipes, and optional GLB models. Add multiple definitions to the pack panel and export one multi-item pack JSON file; import that file with the operator-only in-game definition importer.

The optional JavaScript feature module is an object literal with `onUse`, `onHit`, `onBlockBreak`, `onBlockPlace`, and `onTick` hooks. For example:

```js
({
  onUse(api, event) {
    api.effects.playSound('pickup');
  },
  onHit(api, event) {
    return { damageMultiplier: 1.5, knockbackMultiplier: 1.2 };
  },
  onBlockBreak(api, event) {
    api.inventory.giveItem(110, 1);
  }
})
```

Hooks receive item, player, and event context. The available APIs are `api.inventory.giveItem(id, count)`, `api.world.setBlock(x, y, z, id)`, `api.world.spawnDrop(x, y, z, id, count)`, `api.player.heal(amount)`, `api.player.damage(amount)`, `api.player.addVelocity(x, y, z)`, `api.effects.playSound(name)`, and `api.effects.particle(name, x, y, z)`. `onHit` may return damage and knockback multipliers. Each player must approve a script before it runs; it executes in an isolated worker with no access to the game page or network, a 500 ms time limit, bounded game actions, and in-world position checks. Online PvP healing is applied by the server and rate-limited.

## Render

Do not put the operator code in the source code or in a public Render start command. In the Render dashboard, open your service's **Environment** settings and add a secret environment variable named `EZZYCRAFT_OP_CODE`. Set its value to a private code, save the change, and redeploy the service. The in-game **Options...** screen must be opened from that Render server's URL; enter the same value to unlock the import button.

The server also uses Render's `PORT` environment variable for its listen port. To keep imported definition files across redeploys, attach a persistent disk and set `-definitions-dir` to a folder on that disk using the service's start command.
