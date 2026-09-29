# One prepared image serves both the orchestration and activity workers.
FROM golang:1.27.1-alpine AS go-toolchain
FROM gimbal-browser-evaluator:claude-pi
USER root
RUN apt-get update && apt-get install -y --no-install-recommends docker.io && rm -rf /var/lib/apt/lists/*
COPY --from=go-toolchain /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOCACHE=/tmp/go-build
COPY bin/instrumented-linux /usr/local/bin/instrumented
ENTRYPOINT ["/usr/local/bin/instrumented"]
