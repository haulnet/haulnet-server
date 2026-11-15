# Third-party notices

Components that haulnet-server redistributes, with their licenses. Full license texts are included in
release packages and in the Docker image (`/opt/haulnet/`) next to the component.

| Component | Version | License | Source | Redistributed in |
|---|---|---|---|---|
| LuxonServer | pinned in `luxon.lock` (release binaries of Haulnet's fork, SHA-256 checked) | BSD-3-Clause | https://github.com/haulnet/LuxonServer (fork of https://github.com/niansa/LuxonServer) | Docker image, Linux tarball, Windows zip, as `luxon_server` with `LuxonServer-LICENSE.txt` |
| Go standard library and runtime | the Go version in `go.mod` / the Dockerfile | BSD-3-Clause | https://go.dev | Compiled into `haulnetd` |
| Alpine Linux base image | `ALPINE_VERSION` in the Dockerfile | various (package licenses in the image) | https://alpinelinux.org | Docker image |

LuxonServer is an independent open-source project and is not affiliated with Haulnet.
"R.E.P.O." and "Photon" are named only to describe compatibility.
