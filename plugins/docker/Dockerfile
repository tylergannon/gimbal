FROM node:22-bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends zsh git python3 ca-certificates && rm -rf /var/lib/apt/lists/*
COPY bin/gimbal-linux /usr/local/bin/gimbal
# Consumers may extend this image with additional native agent executables.
ENTRYPOINT ["/usr/local/bin/gimbal"]
