# Porting checklist

This document tracks endpoint coverage. Items marked **done** are implemented.

## Core library (`incl/lib/`)

| PHP file | Go location | Status |
|----------|-------------|--------|
| `connection.php` | `internal/config/config.go` | done |
| `exploitPatch.php` | `internal/sanitize/sanitize.go` | done |
| `XORCipher.php` | `internal/crypto/xor.go` | done |
| `generateHash.php` | `internal/crypto/hash.go` | done |
| `generatePass.php` | `internal/crypto/gjp.go`, `internal/service/auth.go` | done |
| `GJPCheck.php` | `internal/service/gjp.go` | done |
| `ip_in_range.php` | `internal/netutil/ip_range.go` | done |
| `mainLib.php` (getIP, getUserID, getUserString, account lookups) | `internal/netutil/ip.go`, `internal/service/gjp.go`, `internal/service/permissions.go`, `internal/gdlib/format.go` | done |
| `mainLib.php` (permissions, roles, mod actions) | `internal/service/permissions.go`, `internal/service/mods.go` | done |
| `mainLib.php` (mod IP permissions) | `internal/service/permissions.go` (`CheckModIPPermission`) | done |
| `mainLib.php` (Discord PM) | `internal/discord/discord.go` | done |
| `commands.php` | `internal/service/commands.go` | done |
| `Captcha.php` | `internal/captcha/captcha.go` | done |
| `songReup.php` | `internal/service/songs.go` | done |

## Accounts (`accounts/`)

| Endpoint | Status |
|----------|--------|
| `registerGJAccount` | done |
| `loginGJAccount` | done |
| `backupGJAccount` | done |
| `syncGJAccount` / `syncGJAccount20` | done (no defuse-crypto encrypted saves) |

## Levels (`incl/levels/`)

| Endpoint | Status |
|----------|--------|
| `getGJLevels` | done (core search types) |
| `uploadGJLevel` | done |
| `downloadGJLevel` | done (daily/weekly/event, friends-only unlisted) |
| `deleteGJLevelUser` | done |
| `updateGJDesc` | done |
| `reportGJLevel` | done |
| `getGJDailyLevel` | done |
| `rateGJStars` / `rateGJDemon` / `suggestGJStars` | done |

## Scores (`incl/scores/`)

| Endpoint | Status |
|----------|--------|
| `updateGJUserScore` | done |
| `getGJScores` | done |
| `getGJLevelScores` | done |
| `getGJLevelScoresPlat` | done |
| `getGJCreators` | done |

## Social (`incl/relationships/`)

| Endpoint | Status |
|----------|--------|
| `uploadFriendRequest` | done |
| `acceptGJFriendRequest` | done |
| `removeGJFriend` | done |
| `getGJFriendRequests` | done |
| `readGJFriendRequest` | done |
| `deleteGJFriendRequests` | done |
| `blockGJUser` / `unblockGJUser` | done |
| `getGJUserList` | done |

## Comments (`incl/comments/`)

| Endpoint | Status |
|----------|--------|
| `getGJComments` | done |
| `uploadGJComment` | done |
| `deleteGJComment` | done |
| `getGJAccountComments` / `uploadGJAccComment` / `deleteGJAccComment` | done |

## Misc (`incl/misc/`, root)

| Endpoint | Status |
|----------|--------|
| `getCustomContentURL` | done |
| `getAccountURL` | done |
| `likeGJItem` / `likeGJLevel` | done |
| `getGJSongInfo` | done (DB + boomlings fallback) |
| `getGJTopArtists` | done |
| `getGJRewards` | done |
| `getGJChallenges` | done |
| `requestUserAccess` | done |

## Level packs (`incl/levelpacks/`)

| Endpoint | Status |
|----------|--------|
| `getGJMapPacks` | done |
| `getGJGauntlets` | done |
| `getGJLevelLists` | done |
| `uploadGJLevelList` | done |
| `deleteGJLevelList` | done |

## Messages (`incl/messages/`)

| Endpoint | Status |
|----------|--------|
| `getGJMessages` | done |
| `uploadGJMessage` | done |
| `downloadGJMessage` | done |
| `deleteGJMessages` | done |

## Profiles (`incl/profiles/`)

| Endpoint | Status |
|----------|--------|
| `getGJUserInfo` | done |
| `getGJUsers` | done |
| `updateGJAccSettings` | done |

## Web tools (`tools/`, `dashboard/`)

| Tool | Status |
|------|--------|
| `tools/account/activateAccount` | done |
| `tools/account/registerAccount` | done |
| `tools/account/changePassword` | done (no defuse-crypto save re-encryption) |
| `tools/account/changeUsername` | done |
| `tools/index` | done |
| `tools/songAdd` | done |
| `tools/leaderboardsBan` / `leaderboardsUnban` | done |
| `tools/linkAcc` | done |
| `tools/cron/cron` | done |
| `tools/stats/*` | done |
| `tools/bot/*` | done |
| `tools/packCreate` | done |
| `tools/levelReupload` | done |
| `tools/levelToGD` | done |
| `tools/addQuests` | done |
| `tools/revertLikes` | done |
| `tools/cleanup/deleteUnused` | done |
| Dashboard | done |

## PHP parity fixes (audit)

- `getGJCommentHistory` routed to `getGJComments` handler
- `database/accounts/*` backup/sync paths routed
- `updateGJUserScore` returns userID; dinfo/sinfo/pinfo + actions log
- `getGJLevelLists` diff filters and types 7/27 match PHP
- `suggestGJStars` uses `suggestLevelId` column name
- `discordLinkResetPass` uses bcrypt + `RandomString` like PHP
- `getGJLevels` expanded: friends, dailies, lists, songs block, filters
- Individual cron jobs routable at `/tools/cron/{job}`
- `syncGJAccount20` defuse-crypto decrypt on sync (returns `-3` on wrong key)
- `getGJLevels` ID search friend-gate for `unlisted > 1`
- `revertLikes` full implementation (PHP code behind `exit` was ported)

## Notes

- Response formats must match PHP exactly (colon-delimited strings, SHA1 hashes).
- Endpoint paths include `.php` suffix for client compatibility.
- Level binary data is stored on disk in `data/levels/`.
- When adding endpoints: add service logic, handler method, and route in `internal/handler/handler.go`.
