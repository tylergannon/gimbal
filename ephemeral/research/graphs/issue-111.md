# Two runs of one workflow in the same second collide

URL: https://github.com/tylergannon/gimble/issues/111
State: closed
Updated: 2026-09-14T00:43:43Z

Run ids are `YYYYMMDD-HHMMSS.<name>`. Two runs of the same name in one project, started within the same second, collide on the run directory, and the second `Run` fails at `os.Mkdir`.

This is dumb just use https://github.com/oklog/ulid for run ids and call it good.
