FROM ghcr.io/actions/actions-runner:2.337.0
USER root
RUN apt-get update && apt-get install --yes zsh build-essential pkg-config gh \
    && rm -rf /var/lib/apt/lists/* \
    && userdel ubuntu \
    && usermod --uid 1000 runner && groupmod --gid 1000 runner \
    && chown -R runner:runner /home/runner \
    && mkdir -p /home/runner/go/pkg /home/runner/.cache \
        /home/runner/.local/share/pnpm /home/runner/.local/share/vite-plus \
        /home/runner/.config/vite-plus /home/runner/.local/state/vite-plus \
    && chown -R runner:runner /home/runner/go /home/runner/.cache /home/runner/.local /home/runner/.config
USER runner
ENV GOPATH=/home/runner/go GOCACHE=/home/runner/.cache/go-build
WORKDIR /home/runner
ENTRYPOINT ["/home/runner/run.sh"]
