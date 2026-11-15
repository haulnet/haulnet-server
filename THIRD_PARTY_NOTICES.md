# Third-party notices

Components that haulnet-server redistributes, with their licenses. Full license texts are included in
release packages and in the Docker image (`/opt/haulnet/`) next to the component.

| Component | Version | License | Source | Redistributed in |
|---|---|---|---|---|
| LuxonServer | pinned in `luxon.lock` (unmodified release binaries, SHA-256 checked) | BSD-3-Clause | https://github.com/niansa/LuxonServer | Docker image, Linux tarball, Windows zip, as `luxon_server` with `LuxonServer-LICENSE.txt` and `LuxonServer-THIRD_PARTY_NOTICES.txt` |
| Go standard library and runtime | the Go version in `go.mod` / the Dockerfile | BSD-3-Clause | https://go.dev | Compiled into `haulnetd` |
| Alpine Linux base image | `ALPINE_VERSION` in the Dockerfile | various (package licenses in the image) | https://alpinelinux.org | Docker image |

The LuxonServer binaries contain these libraries. Their notices and full license texts ship next to the binary as
`LuxonServer-THIRD_PARTY_NOTICES.txt`, one file per build (sources: `dist/LuxonServer-THIRD_PARTY_NOTICES.linux.txt`
for the Linux musl static binary in the tarball and the image, `dist/LuxonServer-THIRD_PARTY_NOTICES.windows.txt` for
the Windows MSVC binary in the zip):

| Component | Version | License | Source | In build |
|---|---|---|---|---|
| Luxon | commit `81458d7` | BSD-3-Clause | https://gitlab.com/luxon_project/Luxon | Linux, Windows |
| Mbed TLS | 3.6.5 | Apache-2.0 (chosen from Apache-2.0 OR GPL-2.0-or-later) | https://github.com/Mbed-TLS/mbedtls | Linux, Windows |
| spdlog | 1.17.0 | MIT | https://github.com/gabime/spdlog | Linux, Windows |
| JSON for Modern C++ | 3.12.0 | MIT | https://github.com/nlohmann/json | Linux, Windows |
| mini-yaml | as bundled | MIT | https://github.com/jimmiebergmann/mini-yaml | Linux, Windows |
| sqlite3pp | 1.0.10 | MIT | https://github.com/iwongu/sqlite3pp | Linux, Windows |
| magic_enum | 0.9.7 | MIT | https://github.com/Neargye/magic_enum | Linux, Windows |
| Boost.PFR | boost-1.91.0 | BSL-1.0 | https://github.com/boostorg/pfr | Linux, Windows |
| Chromium `flat_map` (adapted in Luxon) | — | BSD-3-Clause | https://chromium.googlesource.com/chromium/src/ | Linux, Windows |
| commoncpp | 1.2.1 | Unlicense | https://gitlab.com/niansa/commoncpp | Linux, Windows |
| SQLite | 3.51.2 | Public domain | https://sqlite.org/ | Linux, Windows |
| wepoll | as bundled | BSD-2-Clause | https://github.com/piscisaureus/wepoll | Windows |
| embedresource | 0.0.12 | BSD-3-Clause | https://github.com/ankurvdev/embedresource | Windows |
| musl libc | 1.2.6 | MIT | https://musl.libc.org/ | Linux |
| GCC runtime libraries (libstdc++, libgcc) | 15.2.0 | GPL-3.0-or-later WITH GCC-exception-3.1 | https://gcc.gnu.org/ | Linux |
| Ryu (inside libstdc++) | as in GCC 15.2.0 | BSL-1.0 (chosen from Apache-2.0 OR BSL-1.0) | https://github.com/ulfjack/ryu | Linux |
| UBSan runtime (GCC libsanitizer, from LLVM compiler-rt) | as in GCC 15.2.0 | NCSA OR MIT | https://gcc.gnu.org/ | Linux |

LuxonServer is an independent open-source project and is not affiliated with Haulnet.
"R.E.P.O." and "Photon" are named only to describe compatibility.
