FROM alpine:3.23.2 AS build

# Install timezone data and Go compiler.
RUN apk add --no-cache tzdata go

# Support specifying Sriracha version.
ARG version

# Copy sriracha source.
COPY . /usr/src/sriracha

# Build sriracha and plugins.
RUN cd /usr/src/sriracha/cmd/sriracha && echo "Fetching dependencies..." && go mod download && echo "Building sriracha..." && go build -trimpath -ldflags "-s -w -X 'codeberg.org/tslocum/sriracha.SrirachaVersion=${version:-DEV}'" && for dir in /usr/src/sriracha/plugin/*; do (echo "Building plugin $(basename $dir)..." && cd "$dir" && go build -trimpath -ldflags "-s -w" -buildmode=plugin); done

FROM alpine:3.23.2

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
