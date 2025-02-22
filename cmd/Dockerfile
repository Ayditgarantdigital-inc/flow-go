# syntax = docker/dockerfile:experimental
# NOTE: Must be run in the context of the repo's root directory

FROM golang:1.15-buster AS build-setup

RUN apt-get update
RUN apt-get -y install cmake zip sudo
# add the pubkey of github.com to knownhosts, so ssh-agent doesn't bark
RUN mkdir -p /root/.ssh && ssh-keyscan -t rsa github.com >> /root/.ssh/known_hosts

## (1) Build Relic first to maximize caching
FROM build-setup AS build-relic

RUN mkdir /build
WORKDIR /build

# Copy over *only* files necessary for Relic
COPY crypto/relic ./relic
COPY crypto/relic_build.sh .

# Build Relic (this places build artifacts in /build/relic/build)
# NOTE: The Relic build script uses Bash-specific features, so we explicitly run
# it with bash rather than the default shell.
RUN bash ./relic_build.sh

## (2) Build the app binary
FROM build-setup AS build-env

# Build the app binary in /app
RUN mkdir /app
WORKDIR /app

ARG TARGET
ARG COMMIT
ARG VERSION

COPY . .

# Copy over Relic build artifacts
COPY --from=build-relic /build/relic/build ./crypto/relic/build

FROM build-env as build-production
WORKDIR /app

# Keep Go's build cache between builds.
# https://github.com/golang/go/issues/27719#issuecomment-514747274
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build --tags "relic,netgo" -ldflags "-extldflags -static" -o ./app ./cmd/${TARGET}

RUN chmod a+x /app/app

## (3) Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base-debian10 as production

COPY --from=build-production /app/app /bin/app

ENTRYPOINT ["/bin/app"]


FROM build-env as build-debug
WORKDIR /app
RUN --mount=type=ssh \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build --tags "relic,netgo" -ldflags "-extldflags -static"  -gcflags="all=-N -l" -o ./app ./cmd/${TARGET}

RUN chmod a+x /app/app

FROM golang:1.15-buster as debug

RUN go get -u github.com/go-delve/delve/cmd/dlv

COPY --from=build-debug /app/app /bin/app

ENTRYPOINT ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/bin/app", "--"]

FROM build-env as build-transit-production-nocgo
WORKDIR /app

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags -static \
    -X 'github.com/onflow/flow-go/cmd/bootstrap/build.commit=${COMMIT}' -X  'github.com/onflow/flow-go/cmd/bootstrap/build.semver=${VERSION}'" \
    -o ./app ./cmd/${TARGET}

RUN chmod a+x /app/app

## (3) Add the statically linked binary to a distroless image
FROM gcr.io/distroless/base-debian10 as production-transit-nocgo

COPY --from=build-transit-production-nocgo /app/app /bin/app

ENTRYPOINT ["/bin/app"]
