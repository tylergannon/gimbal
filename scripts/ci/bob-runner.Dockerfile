FROM ghcr.io/actions/actions-runner:2.337.0
USER root
RUN apt-get update && apt-get install --yes zsh build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/* \
    && userdel ubuntu \
    && usermod --uid 1000 runner && groupmod --gid 1000 runner \
    && chown -R runner:runner /home/runner \
    && mkdir -p /home/runner/go /home/runner/.cache /home/runner/.local/share/pnpm \
    && chown -R runner:runner /home/runner/go /home/runner/.cache /home/runner/.local
USER runner
ENV GOPATH=/home/runner/go GOCACHE=/home/runner/.cache/go-build
WORKDIR /home/runner
ENTRYPOINT ["/home/runner/run.sh"]
