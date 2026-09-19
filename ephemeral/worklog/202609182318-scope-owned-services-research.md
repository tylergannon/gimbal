decision: Use dedicated process-group signaling as the cross-platform baseline; treat descendants that call setsid/setpgid as outside the portable cleanup guarantee, supported by POSIX and Apple/Linux primary sources.
decision: Record Linux child-subreaper and cgroup.kill only as non-portable evidence for the unresolved boundary; do not broaden issue 276 into Linux-specific process management.
