# zsh ownership details

Sources: [zsh invocation](../sources/zsh-invocation.html), [zsh builtins](../sources/zsh-builtins.html), [zsh jobs and signals](../sources/zsh-jobs-signals.html), and [POSIX shell language](../sources/posix-shell-command-language.html).

- zsh `-c` consumes the first argument as shell code. An external command is executed in a separate utility environment; the wrapper remains relevant unless the command explicitly uses zsh `exec`, which replaces the current shell rather than forking.
- zsh `exit` uses the last command's status when no explicit status is supplied; `wait` reports the waited process's status. POSIX gives the same last-simple-command rule, 126/127 command lookup conventions, and reports signal termination above 128.
- A foreground simple command is normally waited for, but shell syntax changes the boundary: an asynchronous list returns immediately (POSIX requires its status to be zero), a pipeline reports the last command, and zsh documents asynchronous jobs, disowning, process substitutions, and shell exit without waiting in specific cases.
- zsh documents inherited signal dispositions, its own special handling of QUIT, and job-control/HUP behavior; these are not a guarantee that a non-interactive `zsh -c` forwards a later SIGTERM to every descendant.

Narrow ownership consequence: Gimbal can observe the shell/foreground command's status and output, but must not infer that shell exit means every descendant is gone. A command that backgrounds, disowns, daemonizes, or delegates to another tool can outlive that status boundary. The source set does not establish a portable default process group/session for `zsh -c`; descendant signaling therefore needs an explicit implementation constraint and live macOS/Linux evidence.
