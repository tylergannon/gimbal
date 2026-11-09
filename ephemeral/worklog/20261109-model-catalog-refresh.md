# Model catalog refresh

discovery: Main still resolved fable to Claude Fable 5.1 and terra to GPT-5.6 Terra. The fresh models.dev catalog contains Fable 5.2 and Terra 6.1; the Diffusion Router chat catalog and limits are unchanged.

decision: Refresh generated data through just models with the existing Doppler gimbal/stg credential. Update active same-tier role defaults, research candidates, examples and desktop-worker guidance; preserve historical fixtures and explicit older-version alias support.

friction: The root Gimbal checkout has unrelated local edits and is behind origin/main. Use an isolated worktree from fetched origin/main; leave that checkout intact.

review: Independent validation found two additional Sonnet 5 defaults in the interview example; both now select Sonnet 5.5. No further active same-tier defaults were found.

friction: Fresh-worktree generation and broad CLI tests require installed SvelteKit metadata and the built SKGO manifest. Run just build-assets before CLI tests; generation alone cannot supply the web bundle. The asset build completed successfully.

friction: The inherited GH_TOKEN is invalid; env -u GH_TOKEN -u GITHUB_TOKEN gh uses the working stored GitHub authentication.

decision: All fourteen existing shorthand families were checked against fresh source catalogs (Gemini flash checked separately against Google). Gemini 4 Argon is a separate new model line in Google documentation, absent from models.dev and not a new Flash or Pro version; this refresh does not invent an alias or replace the chosen Pro tier.

correction: User rejected updating versioned role pointers: roles and examples must always name aliases, so a model upgrade changes exactly one central location. The earlier refresh guidance explicitly instructed repinning defaults; replaced that guidance and the automated PR instructions. Converted all active roles/examples/candidate lists and native boundary defaults to aliases; added Gemini pro centrally and a standalone central resolver for the JS desktop boundary. Initial narrow validation missed this architectural requirement.
