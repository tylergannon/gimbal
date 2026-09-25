# Runs loading feedback

decision: Continue original UI bundle 3 with issue 272; the already-delivered card design resolves the table overflow. Preserve that behavior and address delayed client navigation feedback.
decision: User requested Gimbal implement with current defaults. Installed help confirms Sol high for coding and QA, Astra high for planning, Astra medium for critique. No model overrides.
friction: Prior workflow browser transcript dumps recursively amplified saved session logs. Use bounded output and small fixtures, never dump the current workflow detail transcript.
correction: Scope skeleton CSS under its owning component. Generic global classes such as status and summary collide with RunsList and Topbar even after the loading view disappears.
decision: Managed browser tests use a controlled fixture project; preserve BASE_URL mode for the original server-agnostic scenarios and explicitly skip fixture-dependent cases there.
friction: The coding agent launched a nested independent validator even though implement automatically provides fresh Sol QA. Parent steered it to collect any useful result and finish, avoiding duplicate full review loops.
friction: The staged-content hook rejects table-named JSON even for synthetic test fixtures. Express fixture data in ordinary test setup code and create the tables only at runtime; do not relax the repository policy.
