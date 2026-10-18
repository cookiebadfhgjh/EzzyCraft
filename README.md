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

JavaScript is split into two editors. **Unique behavior module** contains this item's hooks; **Shared JavaScript services** contains reusable helper functions available to all of those hooks through `api.services`. For example:

```js
({
  onUse(api, event) {
    return api.services.playSoundAndLaunch();
  },
  onHit(api, event) {
    const victim = event.attackedPlayer;
    if (victim) api.effects.playSound('hurt');
    return { damageMultiplier: 1.5, knockbackMultiplier: 1.2 };
  },
  onBlockUse(api, event) {
    console.log(event.clickedBy.name + ' used this block');
  },
  onProjectileHit(api, event) {
    if (event.hit.type === 'block') api.effects.particle('break', event.hit.position.x, event.hit.position.y, event.hit.position.z);
  },
  onTick(api, event) {
    if (api.world.time > 18) api.effects.playSound('pickup');
  }
})
```

The shared-services editor can define named functions, for example:

```js
({
  playSoundAndLaunch(api, event) {
    api.effects.playSound('pickup');
    return api.projectiles.launch({ speed: 18, lifetime: 4 });
  }
})
```

Call it from any unique hook with `api.services.playSoundAndLaunch()`. The active hook's API and event are automatically passed as the first two service arguments, followed by any arguments supplied to the service call. Unique hooks include `onUse`, `onHit`, `onBlockUse`, `onBlockBreak`, `onBlockPlace`, `onProjectileHit`, and `onTick`. Block-use events include `event.clickedBy`; hit events include `event.attackedPlayer`. Hooks also receive snapshots through `event.players`, and `api.players.list()` / `api.players.get(id)`.

Available APIs include `api.inventory.giveItem(id, count)`, `api.world.setBlock(x, y, z, id)`, `api.world.spawnDrop(x, y, z, id, count)`, `api.world.setTime(hours)`, `api.world.time`, `api.world.raycast(maxDistance)`, `api.projectiles.launch({ speed, lifetime, direction, origin, damage })`, `api.player.heal(amount)`, `api.player.damage(amount)`, `api.player.addVelocity(x, y, z)`, `api.combat.modifyHit(damageMultiplier, knockbackMultiplier)`, `api.effects.playSound(name)`, and `api.effects.particle(name, x, y, z)`. Raycasts report the nearest aimed-at block or player; projectiles trace blocks and players and invoke `onProjectileHit` on impact. The projectile's optional `damage` value is event metadata; it does not directly damage other players. `onHit` may return damage and knockback multipliers. Time changes in a connected server require operator permission.

Each player must approve the unique code and services before they run. They execute in an isolated worker without access to the game page or network. The runtime allows larger modules and more actions than before, while retaining a two-second hook timeout and checks on world, inventory, health, and server-authoritative operations. Shared server definitions can therefore distribute code, but each player still decides whether to execute it.

## Render

Do not put the operator code in the source code or in a public Render start command. In the Render dashboard, open your service's **Environment** settings and add a secret environment variable named `EZZYCRAFT_OP_CODE`. Set its value to a private code, save the change, and redeploy the service. The in-game **Options...** screen must be opened from that Render server's URL; enter the same value to unlock the import button.

The server also uses Render's `PORT` environment variable for its listen port. To keep imported definition files across redeploys, attach a persistent disk and set `-definitions-dir` to a folder on that disk using the service's start command.
