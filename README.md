# [mojira.dev](https://mojira.dev)
> A mirror of the Minecraft bug tracker written in Go

<div align="center"><img width="600" src="https://raw.githubusercontent.com/misode/mojira.dev/main/images/homepage.png" alt="Homepage of mojira.dev"></div>

## Why do we need this?
Since the [migration](https://minecraft.wiki/w/Bug_tracker#Migration) the public bug tracker has been very slow and unfriendly to work with. There are two official platforms: [bugs.mojang.com](https://bugs.mojang.com) and [report.bugs.mojang.com](https://report.bugs.mojang.com). Both platforms expose part of an issue's metadata, but getting the full picture is difficult.

## How does this work?
The Go server uses the public, servicedesk, and legacy APIs to mirror issues. There are currently 3 systems in place to make sure issues are as much in-sync as possible:

1. A full scan of issues sometimes runs in the background. With currently around 590000 issue keys, this process can take around 4 days.
2. The server actively polls a list of recently updated issues every few seconds and adds them to a queue, which is later processed.
3. Whenever an issue is requested in the frontend and it hasn't been synced within the last 5 minutes, it refreshes the issue.

<div align="center"><img width="600" src="https://raw.githubusercontent.com/misode/mojira.dev/main/images/mc-4.png" alt="Issue detail page"></div>

## Sync queue management
This is mostly internal documentation for myself, but it might be useful to you.

1. Get statistics on the reason and counters in the sync queue
```sql
SELECT reason, failed_count, COUNT(*) FROM sync_queue GROUP BY reason, failed_count;
```

2. Add a single key to the queue
```sql
INSERT INTO sync_queue (issue_key, priority, queue_reason) VALUES ('MC-10000', 5, 'manual')
```

3. Queue all MC issues in a range if they match a condition
```sql
INSERT INTO sync_queue (issue_key, priority, failed_count, reason)
SELECT 'MC-' || i, 1, 0, 'manual-scan'
FROM generate_series(1, 280229) AS i
WHERE EXISTS (
  SELECT 1 FROM issue
  WHERE key = 'MC-' || i AND (status != 'Open' AND status != 'Reopened')
) ON CONFLICT DO NOTHING;
```

4. Queue all resolved MC issues in a range to be synced
```sql
INSERT INTO sync_queue (issue_key, priority, failed_count, reason)
SELECT 'MC-' || i, 1, 0, 'manual-scan'
FROM generate_series(1, 280229) AS i
WHERE EXISTS (
  SELECT 1 FROM issue
  WHERE key = 'MC-' || i AND (status != 'Open' AND status != 'Reopened')
) ON CONFLICT DO NOTHING;
```

5. Queue all WEB issues in a range that aren't mirrored yet
```sql
INSERT INTO sync_queue (issue_key, priority, failed_count, reason)
SELECT 'WEB-' || i, 1, 0, 'manual'
FROM generate_series(1, 8030) AS i
WHERE NOT EXISTS (
  SELECT 1 FROM issue WHERE key = 'WEB-' || i
);
```

6. Queue all issues that are mirrored but marked as removed
```sql
INSERT INTO sync_queue (issue_key, priority, failed_count, reason)
SELECT key, 2, 0, 'removed-check'
FROM issue
WHERE state = 'removed';
```
