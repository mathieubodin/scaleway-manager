# Build stage: cross-compiles the static server binary on the build host's platform.
FROM golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS build
ARG TARGETOS
ARG TARGETARCH
ENV GOTOOLCHAIN=local
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /out/scaleway-manager ./cmd/scaleway-manager

# Runtime stage: distroless, no shell, non-root.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:516757106de35205efb70b91ba03f6f1298424a0678c603255cf2852a7000087
COPY --from=build /out/scaleway-manager /scaleway-manager
USER 65532
ENTRYPOINT ["/scaleway-manager"]
