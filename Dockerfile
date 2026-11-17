# Haulnet server: unmodified LuxonServer + haulnetd (config from env, status endpoint).
# Values must match luxon.lock; CI checks this.
ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
ARG LUXON_VERSION=v1.5
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.luxonVersion=${LUXON_VERSION}" \
      -o /out/haulnetd ./cmd/haulnetd \
 && cp "$(go env GOROOT)/LICENSE" /out/Go-LICENSE.txt

FROM alpine:${ALPINE_VERSION} AS luxon
ARG LUXON_VERSION=v1.5
ARG LUXON_LINUX_ASSET=luxon_server.musl-static.release.linux-x86_64
ARG LUXON_LINUX_SHA256=012d55eec3001918fe988d9315920470842de6305ed4bdc132fabaf817fe6f96
ARG LUXON_LICENSE_SHA256=87ca0810c9a251d379ca432fdcde1b8168e4f172a474b46a32be2386e022e766
RUN wget -q -O /luxon_server "https://github.com/niansa/LuxonServer/releases/download/${LUXON_VERSION}/${LUXON_LINUX_ASSET}" \
 && echo "${LUXON_LINUX_SHA256}  /luxon_server" | sha256sum -c - \
 && wget -q -O /LICENSE "https://raw.githubusercontent.com/niansa/LuxonServer/${LUXON_VERSION}/LICENSE" \
 && echo "${LUXON_LICENSE_SHA256}  /LICENSE" | sha256sum -c - \
 && chmod 755 /luxon_server
# Notices of the libraries inside the binary, written for one LuxonServer release (see scripts/package.sh).
COPY dist/LuxonServer-THIRD_PARTY_NOTICES.linux.txt /NOTICES
RUN head -n 1 /NOTICES | grep -qxF "Third-party notices for luxon_server (LuxonServer ${LUXON_VERSION})" \
 || { echo "LuxonServer-THIRD_PARTY_NOTICES.linux.txt is not written for LuxonServer ${LUXON_VERSION}" >&2; exit 1; }

FROM alpine:${ALPINE_VERSION}
RUN adduser -D -H -u 10001 -s /sbin/nologin haulnet \
 && mkdir -p /var/lib/haulnet && chown haulnet:haulnet /var/lib/haulnet
COPY --from=luxon /luxon_server /opt/haulnet/luxon_server
COPY --from=luxon /LICENSE /opt/haulnet/LuxonServer-LICENSE.txt
COPY --from=luxon /NOTICES /opt/haulnet/LuxonServer-THIRD_PARTY_NOTICES.txt
COPY --from=build /out/haulnetd /out/Go-LICENSE.txt /opt/haulnet/
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
      org.opencontainers.image.description="Haulnet server for R.E.P.O. (LuxonServer)" \
      org.opencontainers.image.licenses="MIT AND BSD-3-Clause"
