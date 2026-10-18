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

Shared definitions are stored in `server_data\definitions`, and complete packs (including their one shared JavaScript module) are stored in `server_data\packs`, next to the server's working directory. The server creates these folders automatically and sends their contents to connected players. Use HTTPS when exposing the server publicly so the operator code is protected in transit.

## Co-op Sprite Studio

Open `Other\coop-sprite-studio.html` to create custom items and blocks. The editor includes combat and mining stats, including a per-tool knockback multiplier, block drops and light, recipes, and optional GLB models. Choose **Pack-wide JavaScript module** from the entry-type dropdown to write and add the pack's one shared module alongside its items and blocks. A pack can also contain JavaScript services and independent game features. The module is stored once under the pack's `javascript.source`; its entry in `entries` identifies it as part of the pack. Export a pack JSON file and import it with the operator-only in-game definition importer.

Each pack has one JavaScript module shared by all its entries. An item's **Pack behavior key** selects its hook object under `items`; JavaScript service entries reference functions under `services`; independent feature entries reference callbacks under `features`:

```js
({
  services: {
    playSoundAndLaunch(game, api, event) {
      api.effects.playSound('pickup');
      return api.projectiles.launch({ speed: 18, lifetime: 4 });
    }
  },
  items: {
    magicSword: {
      onUse(game, api, event) {
        const hits = game.services.rayTrace(12, 5);
        if (hits.some(hit => hit.isPlayer)) api.effects.playSound('pickup');
        return api.services.playSoundAndLaunch();
      },
      onHit(game, api, event) {
        const victim = event.attackedPlayer;
        if (victim) api.effects.playSound('hurt');
        return { damageMultiplier: 1.5, knockbackMultiplier: 1.2 };
      }
    },
    magicBlock: {
      onBlockUse(game, api, event) {
        console.log(event.clickedBy.name + ' used this block');
      }
    }
  },
  features: {
    greeting(game, api, event) {
      if (event.message === 'hello') api.effects.playSound('pickup');
    }
  }
})
```

Use `onUse`, `onHit`, `onBlockUse`, `onBlockBreak`, `onBlockPlace`, `onProjectileHit`, and `onTick` on item behavior objects. Independent feature entries can listen for `onChat`, `onTick`, `onPlayerJoin`, or `onPlayerLeave`. Chat events include `event.sender` and `event.message`; block-use events include `event.clickedBy`; hit events include `event.attackedPlayer`. Event data also includes player snapshots.

The immutable, game-wide `game.services` API is supplied by the runtime and is not pack-editable. `game.services.rayTrace(maxDistance, maxHits)` returns a distance-sorted list of hits from the current view, including `type`, `isBlock`, `isPlayer`, `id`, `distance`, `position`, and block/player details. It can report multiple blocks and players along the ray. Packs can also use `api.services` for their own reusable JavaScript functions.

Hooks receive `(game, api, event)`; pack services receive these arguments followed by any explicit service arguments. The action API includes `api.inventory.giveItem(id, count)`, `api.world.setBlock(x, y, z, id)`, `api.world.spawnDrop(x, y, z, id, count)`, `api.world.setTime(hours)`, `api.world.time`, `api.projectiles.launch({ speed, lifetime, direction, origin, damage })`, `api.player.heal(amount)`, `api.player.damage(amount)`, `api.player.addVelocity(x, y, z)`, `api.combat.modifyHit(damageMultiplier, knockbackMultiplier)`, `api.effects.playSound(name)`, and `api.effects.particle(name, x, y, z)`. `onHit` may return damage and knockback multipliers. Projectile damage is event metadata; online player damage remains server-authoritative. Changing shared server time requires operator permission.

Pack code starts without a per-player approval prompt. It runs in an isolated worker without game-page or network access, with a two-second hook timeout and validated game/server actions. Keep packs under trusted server-operator control.

## Render

Do not put the operator code in the source code or in a public Render start command. In the Render dashboard, open your service's **Environment** settings and add a secret environment variable named `EZZYCRAFT_OP_CODE`. Set its value to a private code, save the change, and redeploy the service. The in-game **Options...** screen must be opened from that Render server's URL; enter the same value to unlock the import button.

The server also uses Render's `PORT` environment variable for its listen port. To keep imported definitions and packs across redeploys, attach a persistent disk and set `-definitions-dir` and `-packs-dir` to folders on that disk using the service's start command.
