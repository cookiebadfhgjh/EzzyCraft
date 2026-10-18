# EzzyCraft

## Starting the server

Build the server with `build.bat`, then start it with an operator code:

```powershell
.\mcserver.exe -op-code "your-private-code"
```

Open the server address in a browser. If you use a separate game page, enter the target server address in **Join Online Game** before opening **Options...**; operator settings and imports use the connected server or that address. Enter the operator code and choose **Unlock Server Settings** to import item, block, or complete pack JSON files. Without `-op-code`, server settings and imports remain disabled.

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

Open `Other\coop-sprite-studio.html` to draw sprites and create custom tools, weapons, blocks, or a multi-entry pack. The entry-type menu switches between **Tool / weapon**, **Placeable block**, and **Pack-wide JavaScript module**. Add entries to the pack as you make them; a pack can contain up to 74 item/block definitions and 256 total entries.

### Drawing, models, and definitions

- Draw with the pen or eraser, choose a color from the palette or color picker, clear the canvas, and undo recent edits.
- Choose a 16×16, 32×32, or 64×64 sprite. Unpainted pixels stay transparent.
- Set the display name, numeric registry ID, internal key, inventory category, and custom atlas tile for a tool or block.
- Add an optional binary glTF `.glb` mesh (up to 20 MB). The editor previews the model and embeds it in exports.
- Define crafting or smelting recipes: shaped patterns, shapeless ingredients, output count, crafting station/grid size, furnace input, and smelting duration.
- Export a definition as standalone HTML, definition JSON, or game-integration JavaScript. Export the current multi-entry pack as pack JSON. The editor can also import definition JSON to continue editing a definition.

### Tool and weapon properties

Tools/weapons have configurable tool type (pickaxe, axe, shovel, hoe, sword, or none), base damage, attack cooldown, knockback multiplier, reach, durability, mining-speed multiplier, harvest level, and maximum stack size. The editor shows a mining-time estimate for the selected example hardness.

### Block properties

Blocks have configurable hardness or unbreakable status, effective tool, effective-tool speed, whether the estimate assumes the effective tool, maximum stack size, drop item ID and count, solidity/collision, transparency, and emitted light. The editor shows mining-time estimates with and without the effective tool.

### Unique behavior for tools and blocks

To give an entry its own behavior, set its **Unique behavior key in the pack module**, for example `magicSword`, and define `items.magicSword` in the shared module. The key is saved with the item/block definition and the JavaScript source is stored once with the pack. A key without a matching object under `items` will not run behavior. Available item/block hooks are:

| Hook | When it runs | Useful event data |
|---|---|---|
| `onUse` | The player uses a selected custom tool/weapon | `event.item`, `event.target` |
| `onHit` | The player attacks another player in online PvP | `event.attackedPlayer`; may return damage/knockback multipliers |
| `onBlockUse` | A player right-clicks a custom block | `event.clickedBy`, block `id`, `position`, and `face` |
| `onBlockBreak` | A custom block is broken, or a custom tool is used to break a block | block `id` and `position`; tool hooks also include `toolId` |
| `onBlockPlace` | A custom block is placed | block `id` and `position` |
| `onProjectileHit` | A projectile from the hook hits a block or player | `event.projectile`, `event.hit`, `event.position`, and, for player hits, `event.attackedPlayer` |
| `onTick` | Each game tick while the custom tool/weapon is held | `event.dt` |

### What is in `event`

Every callback receives `(game, api, event)`. The event is a read-only snapshot describing why the callback ran and the relevant player, item, block, or projectile. It is not a live game object: use `api` to request changes. Properties specific to one hook are listed below; fields shown as optional may be `null` or absent when there is no matching target.

Common context added to **item/block hooks and independent feature callbacks**:

| Property | Contents |
|---|---|
| `event.type` | Event identifier. Values include `item`, `player`, `block-use`, `block`, `projectile-hit`, `tick`, `chat`, `player-join`, and `player-leave`; see the hook tables for which value applies. |
| `event.player` | The local player snapshot: `id`, `name`, `position: { x, y, z }`, `health`, `dead`, `gameMode`; item/block hooks also include `isOperator`, `yaw`, and `pitch`. |
| `event.players` | Array of player snapshots for the local player and known remote players. Item/block-hook remote snapshots include `id`, `name`, `position`, `health`, `dead`, `yaw`, and `pitch`; feature snapshots include `id`, `name`, `position`, `health`, and `dead`. |
| `event.world` | `{ time }`, where `time` is the current in-game time in hours (0–24). |
| `event.rayHits` | Up to 64 current-view ray hits, sorted nearest-first, each within 64 blocks. See the Ray tracing section below. |
| `event.item` | Item/block-hook context only: `{ id, name, kind }` for the definition whose hook is running (`kind` is `tool` or `block`). |

Hook-specific fields:

| Hook | Additional event properties |
|---|---|
| `onUse` | `type: "item"`, `id` is the selected tool ID, and `target` is `null` or `{ type: "block", id, position: { x, y, z } }` for the aimed-at block. |
| `onHit` | `type: "player"`, `id` is the target player ID, `position` is the target position or `null`, and `attackedPlayer` is the target snapshot or `null`. |
| `onBlockUse` | `type: "block-use"`, block `id`, `position: { x, y, z }`, clicked `face: { x, y, z }`, and `clickedBy`, the local player snapshot who right-clicked it. |
| `onBlockBreak` | `type: "block"`, broken block `id`, and `position`. When invoked for the held custom tool, also includes `toolId` for that tool. |
| `onBlockPlace` | `type: "block"`, placed block `id`, and `position`. |
| `onProjectileHit` | `type: "projectile-hit"`, `projectile: { damage, age }`, `hit`, and top-level `position`. A block hit is `{ type: "block", id, position, face }`; a player hit is `{ type: "player", position }` and also supplies `attackedPlayer` with the player's `id`, `name`, `position`, `health`, and `dead` fields. |
| Item/block `onTick` | `type: "tick"` and elapsed frame time `dt` in seconds; runs while that custom tool is held. |
| Feature `onChat` | `type: "chat"`, `sender`, `message`, and `senderPlayer` (known sender snapshot with `id`, `name`, `position`, `health`, `dead`, `yaw`, and `pitch`; `null` if the sender is not a known player). |
| Feature `onTick` | `type: "tick"` and elapsed frame time `dt` in seconds. |
| Feature `onPlayerJoin` | `type: "player-join"` and `joinedPlayer: { id, name }`. |
| Feature `onPlayerLeave` | `type: "player-leave"` and `leavingPlayer: { id, name }`. |

The feature callbacks receive the common player/world/ray-hit context, but do not receive `event.item`. Feature event payloads describe the sender or joining/leaving player separately; for example, a join event's `joinedPlayer` currently contains only ID and name. The online server sends chat and player lifecycle events to clients, so the local feature callback runs on each connected client that has installed the pack. Of all item/block hook return values, only `onHit` uses its return value: return `{ damageMultiplier, knockbackMultiplier }` to adjust a PvP hit. Use `api` for side effects.

#### Reading and using event data

`event.type` tells you which kind of event payload you are handling. Properties that only apply to another hook won't be present on the current event, so check them before using them. For example, `attackedPlayer` is only useful on player-hit events; a player ray hit's `player` field is present only when `type === "player"`:

```js
items: {
  scanner: {
    onUse(game, api, event) {
      const hits = event.rayHits || [];
      for (const hit of hits) {
        if (hit.type === 'block') {
          console.log('Block', hit.id, hit.blockPosition, 'distance', hit.distance, 'face', hit.face);
        } else if (hit.type === 'player') {
          console.log('Player', hit.player.name, hit.player.health, 'distance', hit.distance);
        }
      }
      console.log('Held item:', event.item.name, 'player:', event.player.name, 'time:', event.world.time);
    },
    onHit(game, api, event) {
      const victim = event.attackedPlayer;
      if (!victim) return;
      console.log('Hit', victim.name, 'at', event.position, 'health:', victim.health);
      return { damageMultiplier: 1.25, knockbackMultiplier: 1.5 };
    }
  }
},
features: {
  chatLogger(game, api, event) {
    if (event.type === 'chat') {
      console.log(event.sender, event.message, event.senderPlayer);
    }
  }
}
```

In particular:

- `event.position` has different meanings depending on the hook: for `onHit` it is the attacked player's position; for block and projectile hooks it is the relevant block/hit position.
- `event.id` is also hook-specific: it is a player ID for `onHit`, a selected item ID for `onUse`, and a block ID for block hooks.
- In `onBlockBreak`, a custom tool hook's `event.id` is the block that was broken; `event.toolId` is the held tool's ID.
- `event.rayHits` is computed for the current view when the hook is called. It can include solid blocks and remote living players, sorted by distance. A block hit has `{ type: "block", isBlock: true, isPlayer: false, id, distance, position, blockPosition, face }`. A player hit has `{ type: "player", isBlock: false, isPlayer: true, id, distance, position, player }`, where `player` contains `id`, `name`, `health`, `dead`, and `position`.
- `event.players` is a snapshot for lookup and inspection, not a live handle. The player list helper `api.players.get(id)` searches that same event snapshot.
- `event.world.time` is a number of in-game hours. Read it to make decisions; use `api.world.setTime(hours)` to request a change.

Example tool and block hooks:

```js
items: {
  magicSword: {
    onUse(game, api, event) {
      const firstHit = game.services.rayTrace(12, 1)[0];
      if (firstHit?.isPlayer) api.effects.playSound('hurt');
      api.projectiles.launch({ speed: 18, lifetime: 4, damage: 3 });
    },
    onHit(game, api, event) {
      if (event.attackedPlayer) {
        return { damageMultiplier: 1.5, knockbackMultiplier: 1.2 };
      }
    }
  },
  magicBlock: {
    onBlockUse(game, api, event) {
      api.effects.playSound('pickup');
    }
  }
}
```

### Pack-wide JavaScript, services, and features

A pack has one module with `items`, `services`, and `features` maps. Select **Pack-wide JavaScript module**, write the module, and choose **Add JavaScript module to pack**. It appears in the pack entry list; switching entry types preserves the draft. The module source is exported once as `javascript.source`; the `javascript` entry in the pack manifest identifies it as a pack entry, it is not duplicated per item. JavaScript service entries refer to functions under `services`. Independent feature entries refer to functions under `features` and can listen for:

- `onChat`: `event.sender`, `event.message`, and `event.senderPlayer`.
- `onTick`: elapsed tick time in `event.dt`.
- `onPlayerJoin`: the joining player in `event.joinedPlayer`.
- `onPlayerLeave`: the departing player in `event.leavingPlayer`.

These callbacks also receive the shared `event.player`, `event.players`, and `event.world` context. Pack services are reusable functions; call them as `api.services.serviceName(args...)`. Their function signature is `(game, api, event, ...args)`. Item hooks and feature callbacks use `(game, api, event)`. One possible module:

```js
({
  items: {
    magicBlock: {
      onBlockUse(game, api, event) {
        api.services.notifyClick(event.clickedBy.name);
      }
    }
  },
  services: {
    notifyClick(game, api, event, playerName) {
      api.effects.playSound('pickup');
      return playerName;
    }
  },
  features: {
    greeting(game, api, event) {
      if (event.message === 'hello') api.effects.playSound('pickup');
    }
  }
})
```

### JavaScript APIs

The runtime provides an immutable, game-wide `game.services.rayTrace(maxDistance, maxHits)`. It returns the current-view hits in distance order; each hit includes `type`, `isBlock`, `isPlayer`, `id`, `distance`, and `position`, plus block or player details. It can return multiple blocks and remote players along the ray. Use it either directly from `game.services` or through `event.rayHits`.

The `api` passed to hooks and services exposes:

| API | What it can do |
|---|---|
| `api.services.name(...args)` | Call a function from this pack's shared `services` object. |
| `api.combat.modifyHit(damageMultiplier, knockbackMultiplier)` | Build a hit-modifier result. An `onHit` hook can return `{ damageMultiplier, knockbackMultiplier }`; the runtime clamps these values. |
| `api.inventory.giveItem(id, count)` | Add a registered item/block to the local player's inventory. |
| `api.world.setBlock(x, y, z, id)` | Set or remove a nearby in-bounds world block, subject to collision and world validation. |
| `api.world.spawnDrop(x, y, z, id, count)` | Spawn a registered item drop near the player. |
| `api.world.setTime(hours)` / `api.world.time` | Request a world time from 0 to under 24 hours, and read the event's current time. Changing shared multiplayer time requires server-operator permission. |
| `api.projectiles.launch(options)` | Launch a visible projectile. Options include `speed`, `lifetime`, `direction`, `origin`, and `damage`; a hit invokes `onProjectileHit`. Damage is event metadata and does not by itself apply damage to another player. |
| `api.player.heal(amount)` / `damage(amount)` | Heal or damage the local player, subject to gameplay and server rules. |
| `api.player.addVelocity(x, y, z)` | Apply a velocity change to the local player. |
| `api.players.list()` / `get(id)` | Read the event's player snapshot(s); these are data, not controls for other players. |
| `api.effects.playSound(name)` | Play a built-in sound effect. |
| `api.effects.particle(name, x, y, z)` | Spawn a supported particle effect (`break`, `smoke`, `bubble`, `water`, or `lava`) near the player. |

#### Raycasting

Call `game.services.rayTrace(maxDistance, maxHits)` inside an item hook or feature. Both arguments are optional: the defaults are the supplied view snapshot's full range and hit count (up to 64 blocks and 64 results). The ray starts at the camera and points where the player is looking. It returns an array sorted nearest-first; `maxDistance` filters the results by distance and `maxHits` limits the number returned. The ray includes non-fluid solid blocks and living remote players; it does not include the local player or fluids. A hit's `distance` is the distance from the camera, and `position` is the point along the ray where it hit.

```js
onUse(game, api, event) {
  // Use the ray snapshot already included with this event.
  const target = event.rayHits.find(hit => hit.type === 'block');
  if (!target) return;
  console.log('Block ID:', target.id);
  console.log('Block coordinates:', target.blockPosition.x, target.blockPosition.y, target.blockPosition.z);
  console.log('Hit face:', target.face);

  // Or request only the nearest hit within 12 blocks.
  const nearest = game.services.rayTrace(12, 1)[0];
  if (nearest?.type === 'player') {
    console.log('Player in sight:', nearest.player.name);
  }
}
```

Each block hit includes `type: "block"`, `isBlock: true`, `isPlayer: false`, the block registry `id`, `distance`, world `position`, integer `blockPosition`, and hit `face`. Each player hit includes `type: "player"`, `isBlock: false`, `isPlayer: true`, player `id`, `distance`, hit `position`, and a `player` snapshot (`id`, `name`, `health`, `dead`, and feet `position`). Check `hit.type` before reading `blockPosition` or `player`; those details apply only to their respective hit type. `event.rayHits` is the precomputed default ray for the event, so use the service when you need a shorter range or fewer results.

#### Projectiles

Launch a projectile from an item hook (usually `onUse`) or feature callback with `api.projectiles.launch(options)`. It creates a visible projectile moving from the camera in the direction the player is looking by default:

```js
items: {
  fireWand: {
    onUse(game, api, event) {
      api.projectiles.launch({
        speed: 18,
        lifetime: 4,
        damage: 3
      });
    },
    onProjectileHit(game, api, event) {
      if (event.hit.type === 'block') {
        console.log('Projectile hit block', event.hit.id, 'at', event.hit.position);
        api.effects.particle('break', event.hit.position.x, event.hit.position.y, event.hit.position.z);
      } else if (event.hit.type === 'player') {
        console.log('Projectile hit player', event.attackedPlayer.name);
      }
    }
  }
}
```

`options` fields:
| Field | Meaning |
|---|---|
| `speed` | Projectile speed; defaults to `16`, accepted range is `0.1`–`80`. |
| `lifetime` | Seconds before the projectile expires; defaults to `5`, accepted range is `0.1`–`30`. |
| `damage` | Value copied into `event.projectile.damage` (defaults to `0`, range `0`–`20`). It is metadata only and does not automatically damage a player or block. |
| `direction` | Optional `{ x, y, z }` direction in world coordinates. If absent or invalid, the view direction is used. The direction is normalized. |
| `origin` | Optional `{ x, y, z }` world position. It is used only if within 8 blocks of the camera; otherwise the projectile starts at the camera. |

The projectile's originating item behavior or feature handler receives its `onProjectileHit` hook when it hits a solid block. Player collisions are checked only in an online game while PvP is enabled; then the event also has `event.attackedPlayer`. The hit event contains `event.projectile: { damage, age }`, `event.hit` (hit type and details), and the impact `event.position`. A projectile expires without a hit callback when its lifetime elapses. To apply an effect on impact, explicitly call an API from `onProjectileHit`, such as `api.effects.playSound`, `api.effects.particle`, or `api.world.setBlock`; `damage` alone never applies damage.

Online player combat and shared time remain server-authoritative. Pack code runs without a per-player approval prompt in an isolated worker without game-page or network access. Hooks have a two-second time limit and actions are validated and bounded. Only import packs from server operators you trust.

## Render

Do not put the operator code in the source code or in a public Render start command. In the Render dashboard, open your service's **Environment** settings and add a secret environment variable named `EZZYCRAFT_OP_CODE`. Set its value to a private code, save the change, and redeploy the service. The in-game **Options...** screen must be opened from that Render server's URL; enter the same value to unlock the import button.

The server also uses Render's `PORT` environment variable for its listen port. To keep imported definitions and packs across redeploys, attach a persistent disk and set `-definitions-dir` and `-packs-dir` to folders on that disk using the service's start command.
