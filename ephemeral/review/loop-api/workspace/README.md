# mergequeue

`mergequeue` reads one dependency-aware merge queue from standard input and
writes its deterministic waves and blocked PR IDs as JSON.

Build and run it with:

```sh
go build -o mergequeue .
printf '%s\n' '{"prs":[{"id":4,"deps":[2,3],"checks":"pass"},{"id":1,"deps":[],"checks":"pass"},{"id":2,"deps":[1],"checks":"pass"},{"id":3,"deps":[1],"checks":"pass"}]}' | ./mergequeue
```

Output:

```json
{"waves":[[1],[2,3],[4]],"blocked":[]}
```

By default, every ready passing PR is included in each wave. Use `--parallel N`
with a positive integer to cap each wave; ready PRs are selected by ascending
ID. For example:

```sh
printf '%s\n' '{"prs":[{"id":3,"deps":[],"checks":"pass"},{"id":1,"deps":[],"checks":"pass"},{"id":2,"deps":[],"checks":"pass"}]}' | ./mergequeue --parallel 2
```

Output:

```json
{"waves":[[1,2],[3]],"blocked":[]}
```

Failed or pending PRs, plus every PR depending on them, appear in `blocked`.
Invalid input prints a diagnostic to standard error, prints no result, and
exits with status 2.
