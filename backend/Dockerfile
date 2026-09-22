# The API and the migrator, in one image.
#
# Both binaries ship together because a deployment needs both and a server
# should not need a Go toolchain to get the second one. The API is the
# entrypoint; the migrator is reached by overriding it:
#
#   docker run --rm --entrypoint /migrate -e DATABASE_URL=... goklay-api up
#
# Distroless with no shell, running as nonroot: there is nothing in the final
# image but two static binaries and CA certificates.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Dependencies first, so a source change does not re-download them. go.sum is
# copied with go.mod rather than arriving later with the source: `go mod
# download` writes one when it is absent, and a generated go.sum that differs
# from the repository's turns a dependency change into a confusing build
# failure instead of a clear one.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
COPY --from=build /out/migrate /migrate
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
