# Authoritative user direction
Read and plan issue 386. Get consensus with Claude Fable that the plan has correct steps, validation, and definition of done. Do not admit proof machinery above and beyond requirements. This is an established harness pattern: good enough is better than perfect. Definition of done: workflows can run against the Diffusion router using the Pi harness.
Run the implement workflow with Sonnet planning and validation and Luna coding. Once running, let it run detached; do not obsessively poll.
The user supplied and authorized a temporary Diffusion API key. It will be available as DIFFUSION_API_KEY in the dedicated instance environment. Do not print its value or put it in tracked files.
