# GitHub commands

Use placeholders `<pr>`, `<owner>`, `<repo>`, and `<base>`. Keep temp files (GraphQL queries, reply bodies) outside the repository working tree, for example under `${TMPDIR:-/tmp}`, so they are never committed.

## Current PR

Read the PR for the current branch:

```sh
gh pr view --json number,url
```

## PR state

Read the target PR:

```sh
gh pr view <pr> --json number,url,state,isDraft,mergeable,mergeStateStatus,reviewDecision,headRefName,headRefOid,baseRefName
```

`reviewDecision` is an empty string when there is no decision.

## Default branch

Read the repository default branch name:

```sh
gh repo view --json defaultBranchRef --jq .defaultBranchRef.name
```

## Owner and repo

Read `owner/repo`:

```sh
gh repo view --json owner,name --jq '.owner.login + "/" + .name'
```

## Checks snapshot

Read check names, states, buckets, and links:

```sh
gh pr checks <pr> --json name,state,bucket,link,workflow
```

`bucket` is one of `pass`, `fail`, `pending`, `skipping`, `cancel`. Exit codes: 0 all passed, 1 some failed, 8 some pending. With `--json`, read stdout even on exit 1 or 8.

## Wait on checks

Block until checks finish:

```sh
gh pr checks <pr> --watch --interval 60
```

## Failing logs

The check `link` looks like `https://github.com/<owner>/<repo>/actions/runs/<run-id>/job/<job-id>`. Read failed logs by job:

```sh
gh run view --job <job-id> --log-failed
```

Or by run:

```sh
gh run view <run-id> --log-failed
```

Non-Actions checks (external apps) have other links: open the link's details instead.

## Rerun once

Rerun only the failed jobs in a run:

```sh
gh run rerun <run-id> --failed
```

## Merge base

Fetch then merge the latest base into the PR branch:

```sh
git fetch origin <base>
git merge origin/<base>
```

Abort when intents genuinely conflict:

```sh
git merge --abort
```

## Unresolved review threads

Save the query below to a file outside the working tree. `-F` reads a value from a file when it starts with `@`, and converts numbers, so `pr` is sent as Int.

```graphql
query($owner: String!, $repo: String!, $pr: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $pr) {
      reviewThreads(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          isOutdated
          path
          line
          comments(first: 20) {
            nodes { author { login } body url createdAt }
          }
        }
      }
    }
  }
}
```

Run the query:

```sh
gh api graphql -F query=@threads.graphql -f owner=<owner> -f repo=<repo> -F pr=<pr>
```

Paginate while `hasNextPage` is true by adding `-f after=<endCursor>`. Filter unresolved threads:

```sh
gh api graphql -F query=@threads.graphql -f owner=<owner> -f repo=<repo> -F pr=<pr> --jq '.data.repository.pullRequest.reviewThreads.nodes[] | select(.isResolved | not)'
```

## Reply to a thread

Write the reply body to a file outside the working tree. Save this mutation to a file:

```graphql
mutation($threadId: ID!, $body: String!) {
  addPullRequestReviewThreadReply(input: { pullRequestReviewThreadId: $threadId, body: $body }) {
    comment { url }
  }
}
```

Send the reply from those files:

```sh
gh api graphql -F query=@reply.graphql -f threadId=<thread-id> -F body=@reply.md
```

## Resolve a thread

Save this mutation to a file:

```graphql
mutation($threadId: ID!) {
  resolveReviewThread(input: { threadId: $threadId }) {
    thread { isResolved }
  }
}
```

Resolve by thread id:

```sh
gh api graphql -F query=@resolve.graphql -f threadId=<thread-id>
```
