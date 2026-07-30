FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/ingestd ./cmd/ingestd \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/normalize ./cmd/normalize \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/replay ./cmd/replay \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/analyse ./cmd/analyse \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/reviewd ./cmd/reviewd \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/privacy-delete ./cmd/privacy-delete \
 && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/retention ./cmd/retention \
 && mkdir -p /out-data/raw /out-data/dayz-spool \
 && chmod 0750 /out-data /out-data/raw /out-data/dayz-spool

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out /app
COPY --from=build --chown=65532:65532 /out-data /data
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/ingestd"]

# DZMap's slim image contains level-4 tiles for many terrains. The review image
# copies only the four maps supported by this repository; no DZMap process runs
# at runtime. The digest pins the map input used by deploy/maps/catalog.json.
FROM ghcr.io/woozymasta/dzmap:slim@sha256:b2b83c2d73bc7e9e823c69462f37bdc410a0fc790deea9ec4cb33351dfc30cd7 AS dzmap-assets

FROM runtime AS review-runtime
COPY --from=dzmap-assets --chown=65532:65532 /maps/chernarusplus /app/maps/chernarusplus
COPY --from=dzmap-assets --chown=65532:65532 /maps/enoch /app/maps/enoch
COPY --from=dzmap-assets --chown=65532:65532 /maps/sakhal /app/maps/sakhal
COPY --from=dzmap-assets --chown=65532:65532 /maps/namalsk /app/maps/namalsk
COPY --chown=65532:65532 deploy/maps/catalog.json /app/maps/catalog.json
COPY --chown=65532:65532 THIRD_PARTY_NOTICES.md /app/THIRD_PARTY_NOTICES.md

FROM runtime AS default
