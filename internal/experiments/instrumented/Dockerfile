# One prepared image serves both the orchestration and activity workers.
FROM gimbal-browser-evaluator:claude-pi
USER root
RUN apt-get update && apt-get install -y --no-install-recommends docker.io && rm -rf /var/lib/apt/lists/*
COPY bin/instrumented-linux /usr/local/bin/instrumented
ENTRYPOINT ["/usr/local/bin/instrumented"]
