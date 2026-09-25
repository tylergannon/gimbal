friction: runlog.Read treated a newly visible run directory as a complete path and returned ENOENT before the writer created run.jsonl -> observers should attach at directory discovery and let the reader wait for the promised log file under their context
friction: just test from a clean worktree lacks the generated skgo.manifest.json and fails the web runtime tests -> run just build before the complete test gate
friction: just vet is currently blocked after go vet and build by five existing gimbal lint findings outside issue 192 -> repair the baseline lint findings separately
friction: TestAttestEventFixture once exceeded its one-second observation deadline during the full suite, then passed 10 of 10 isolated runs and the next full suite -> its timing budget is flaky under suite load
