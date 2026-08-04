# https://hub.docker.com/_/alpine/tags
FROM alpine:3.24.1 AS build

# Install timezone data and Go compiler.
RUN apk add --no-cache tzdata go

# Support specifying Sriracha version.
ARG version

# Fetch dependencies.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    echo "Fetching dependencies..." && go mod download

# Copy sriracha source.
COPY . /usr/src/sriracha

# Build sriracha and custom plugins.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd /usr/src/sriracha/cmd/sriracha && \
    echo "Building sriracha..." && \
    go build -trimpath -ldflags "-s -w -X 'codeberg.org/tslocum/sriracha.SrirachaVersion=${version:-DEV}'" && \
    if [ -d "/usr/src/sriracha/customplugin/" ]; then \
        for dir in /usr/src/sriracha/customplugin/*; do \
            (echo "Building custom plugin $(basename $dir)..." && cd "$dir" && go build -trimpath -ldflags "-s -w" -buildmode=plugin); \
        done; \
    fi && \
    cd /usr/src/sriracha && \
    find . -type f -name '*.go' -delete

FROM alpine:3.24.1

# Set working directory.
WORKDIR /usr/share/sriracha

# Set entrypoint.
ENTRYPOINT [ "sriracha" ]

# Create symbolic link to binary.
RUN ln -s /usr/share/sriracha/cmd/sriracha/sriracha /usr/bin/sriracha

# Install timezone data and ffmpeg.
RUN apk add --no-cache tzdata ffmpeg

# Copy source code and built files.
COPY --from=build --chown=1000:1000 /usr/src/sriracha /usr/share/sriracha
