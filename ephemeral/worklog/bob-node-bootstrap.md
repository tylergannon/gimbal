# Bob Node bootstrap

friction: First Bob release failed before source generation: pnpm 12 installation invokes node from PATH, while the official runner image only carries its private action runtime. Hosted runners had hidden the assumption. Set up Node 24 before pnpm in both Bob jobs; keep Mac setup unchanged. Publication correctly remained blocked.
