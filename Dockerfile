FROM alpine:3.23.2 AS build

# Support specifying Sriracha version.
ARG version

# Install timezone data and Go compiler.
RUN apk add --no-cache tzdata go

# Copy sriracha source.
COPY . /usr/src/sriracha

# Compile sriracha.
RUN cd /usr/src/sriracha/cmd/sriracha && go build -x -ldflags "-s -w -X 'codeberg.org/tslocum/sriracha.SrirachaVersion=${version:-DEV}'"

# Compile plugins.
RUN for dir in /usr/src/sriracha/plugin/*; do (cd "$dir" && go build -x -ldflags "-s -w" -buildmode=plugin); done

FROM alpine:3.23.2

# Install timezone data and ffmpeg.
RUN apk add --no-cache tzdata ffmpeg

# Copy source code and built files.
COPY --from=build /usr/src/sriracha /usr/share/sriracha

# Move binary.
RUN mv /usr/share/sriracha/cmd/sriracha/sriracha /usr/bin/sriracha

# Set owner and group.
RUN chown -R 1000:1000 /usr/share/sriracha
RUN chown -R 1000:1000 /usr/bin/sriracha

# Set working directory.
WORKDIR /usr/share/sriracha

# Set entrypoint.
ENTRYPOINT [ "sriracha" ]
