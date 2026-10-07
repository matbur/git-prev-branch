# Build stage: the Go toolchain only, never shipped.
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Same stamping as `make build`: -X main.version, defaulting to "dev". Pass
# --build-arg VERSION="$(git describe --tags --match 'v[0-9]*' --always)"
# to bake in a real version.
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/git-prev-branch ./cmd/git-prev-branch

# Runtime stage: nothing but git, the binary, and a non-root user.
FROM alpine:3.24

# A dedicated non-root user: the image must be safe to run without --user.
# safe.directory keeps git usable for a repository bind-mounted from the host,
# whose owner is a different, numeric-id-only user inside the container.
RUN apk add --no-cache git \
    && adduser -D -h /home/uid1000 uid1000

COPY --from=build /out/git-prev-branch /usr/local/bin/git-prev-branch

USER uid1000
ENV HOME=/home/uid1000
RUN git config --global --add safe.directory '*'

# The alias in the README mounts the current directory at /repo.
WORKDIR /repo
ENTRYPOINT ["git-prev-branch"]
