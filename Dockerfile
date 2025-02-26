# argument for Go version
ARG GO_VERSION=1.22

# STAGE 1: building the executable
FROM golang:${GO_VERSION}-alpine AS build

ENV CGO_ENABLED=0
ARG BUILD_REF

WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./

# Build the executable
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.build=${BUILD_REF}" \
    -installsuffix 'static' \
    -o /app ./app

# Run the Go Binary in Distroless.
FROM gcr.io/distroless/static@sha256:d6fa9db9548b5772860fecddb11d84f9ebd7e0321c0cb3c02870402680cc315f AS final

# change to a nonroot user
USER nonroot:nonroot

# copy compiled app
COPY --from=build --chown=nonroot:nonroot /app /app

ENTRYPOINT ["./app"]

