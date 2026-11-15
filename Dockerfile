# Haulnet community server: LuxonServer (Haulnet's fork, release binary) + haulnetd (config from env, status endpoint).
# Values must match luxon.lock; CI checks this.
ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
ARG LUXON_VERSION=v1.5-haulnet.1
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.luxonVersion=${LUXON_VERSION}" \
      -o /out/haulnetd ./cmd/haulnetd

FROM alpine:${ALPINE_VERSION} AS luxon
ARG LUXON_REPO=haulnet/LuxonServer
ARG LUXON_VERSION=v1.5-haulnet.1
ARG LUXON_LINUX_ASSET=luxon_server.musl-static.release.linux-x86_64
ARG LUXON_LINUX_SHA256=71c48f9790b35c71ae7f44d36a4e40e0885422db6e8740c2aa6274a328670c74
ARG LUXON_LICENSE_SHA256=87ca0810c9a251d379ca432fdcde1b8168e4f172a474b46a32be2386e022e766
RUN wget -q -O /luxon_server "https://github.com/${LUXON_REPO}/releases/download/${LUXON_VERSION}/${LUXON_LINUX_ASSET}" \
 && echo "${LUXON_LINUX_SHA256}  /luxon_server" | sha256sum -c - \
 && wget -q -O /LICENSE "https://raw.githubusercontent.com/${LUXON_REPO}/${LUXON_VERSION}/LICENSE" \
 && echo "${LUXON_LICENSE_SHA256}  /LICENSE" | sha256sum -c - \
 && chmod 755 /luxon_server

FROM alpine:${ALPINE_VERSION}
RUN adduser -D -H -u 10001 -s /sbin/nologin haulnet \
 && mkdir -p /var/lib/haulnet && chown haulnet:haulnet /var/lib/haulnet
COPY --from=luxon /luxon_server /opt/haulnet/luxon_server
COPY --from=luxon /LICENSE /opt/haulnet/LuxonServer-LICENSE.txt
COPY --from=build /out/haulnetd /opt/haulnet/haulnetd
COPY LICENSE THIRD_PARTY_NOTICES.md /opt/haulnet/
ENV WORK_DIR=/var/lib/haulnet
USER haulnet
EXPOSE 27000-27002/udp 27080/tcp
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s CMD ["/opt/haulnet/haulnetd", "-healthcheck"]
ENTRYPOINT ["/opt/haulnet/haulnetd"]
# haulnet-update installs only images with this source and a release version (X.Y.Z).
ARG VERSION=dev
ARG REVISION=
LABEL org.opencontainers.image.source="https://github.com/haulnet/haulnet-server" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.description="Haulnet community server for R.E.P.O. (LuxonServer)" \
      org.opencontainers.image.licenses="MIT AND BSD-3-Clause"
