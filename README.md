# [mojira.dev](https://mojira.dev)
> A mirror of the Minecraft bug tracker written in Go

<div align="center"><img width="600" src="https://raw.githubusercontent.com/misode/mojira.dev/main/images/mc_4.png" alt="[MC-4] Cracked effect sometimes remains when a zombie is interrupted while attempting to break down a door."></div>

## Why do we need this?
Since the [migration](https://minecraft.wiki/w/Bug_tracker#Migration) the public bug tracker has been very slow and unfriendly to work with. There are two official platforms: [bugs.mojang.com](https://bugs.mojang.com) and [report.bugs.mojang.com](https://report.bugs.mojang.com). Both platforms expose part of an issue's metadata, but getting the full picture is difficult.

## How does this work?
The Go server uses both the public and servicedesk APIs to mirror issues. There are currently 3 systems in place to make sure issues are as much in-sync as possible:

1. A full sweep sync of issues runs in the background. With currently around 590000 issue keys, this process can take around 4 days.
2. The server actively polls a list of recently updated issues every few seconds and adds them to a queue, which is later processed.
3. Whenever an issue is requested in the frontend and it hasn't been synced within the last 5 minutes, it refreshes the issue.
