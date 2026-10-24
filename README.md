# Retro VLESS

An independent, minimal native-dialog VLESS client targeting **Intel macOS High Sierra 10.13.6**. It wraps the official **sing-box 1.3.6** core; it is not a fork of sing-box and has no upstream endorsement. Dialogs and connection messages are currently in Russian; dialog titles are `retro vless`.

## Compatibility and installation

The app contains Intel x86-64 executables with a macOS 10.13 binary deployment minimum. The wrapper is compiled with Go 1.20.x (the last Go release series supporting macOS 10.13), CGO disabled and baseline amd64 instructions. The bundle declares 10.13.6 as its minimum supported system. **Actual execution on a High Sierra Mac has not been verified.** Binary metadata and Linux protocol integration tests are not a substitute for testing macOS routing, authorization and AppleScript on that machine.

1. Unzip `Retro-VLESS-Mac-Intel.zip` and move `Retro VLESS.app` to Applications.
2. This app is unsigned and not notarized. Right-click it and choose **Open**, then approve the macOS warning. Only do this for a download you trust.
3. Choose **добавить ссылку** (add link), paste a `vless://` link or an `https://` subscription URL, then select **подключиться** (connect) and choose a server.
4. macOS requests an **administrator password** when starting or stopping the systemwide TUN connection. The app does not store that password.
5. Choose **отключиться** to disconnect. **выйти** exits after requesting disconnection.

The installed app needs **no Python, Xcode or Go installation**. It uses macOS's built-in AppleScript interpreter and the two bundled native executables. A public IPv4 address for the remote server is required. Another VPN can interfere with routing.

## Supported links

- VLESS over TCP, WebSocket (WS) and gRPC, with TLS or legacy REALITY options supported by sing-box 1.3.6. Plain VLESS TCP is also accepted; it does not provide TLS protection.
- Direct VLESS links and plain-text or base64 subscriptions containing VLESS links. Unsupported subscription entries are skipped; unsupported/duplicate link options are rejected.
- Subscription requests use a V2Ray-compatible User-Agent to request a VLESS link list from providers that otherwise return a sing-box JSON configuration.
- HTTPS subscription URLs are fetched **only when imported or reimported**. There is **no automatic refresh**, and the subscription URL is not retained for periodic polling. Import replaces the saved server list and requires disconnecting first.
- Newer protocols and extensions such as XHTTP, newer REALITY/encryption options, insecure TLS and unsupported fingerprints are not supported.

The bundled core is intentionally old for the compatibility target. It does not include later security fixes. Use only if you accept the risks of both a legacy OS and legacy network software.

## Privacy and limits

This is a systemwide TUN client, not a browser-only proxy. It checks the route and an HTTPS connectivity probe after starting, but those checks **do not prove that every application or packet is protected**. There is **no kill switch**. DNS, local/LAN traffic, existing connections or routing changes can bypass the tunnel; **DNS/LAN leak prevention is not guaranteed**. Losing the tunnel may restore direct connectivity. The connectivity check contacts `https://example.com`; configured tunnel DNS uses Cloudflare DNS over HTTPS.

Imported links contain credentials. They are stored locally in `~/Library/Application Support/Retro VLESS/servers.json` (owner-only permissions); active configuration and session state are also kept in that directory. Never publish these files. Links are hidden in the import dialog, but local administrators or process inspection may still observe command arguments during import. No private subscription or provider configuration is included in this distribution.

## Build from source (Linux or macOS)

Install **Go 1.20.x** and Python 3.8 or later on the build machine. Newer Go versions are deliberately rejected to preserve the legacy target.

```sh
GO=/path/to/go1.20/bin/go python3 build.py
```

`GO` is optional if `go` on PATH is Go 1.20.x. The builder downloads the official `sing-box-1.3.6-darwin-amd64.tar.gz` asset (baseline **amd64, not amd64v3**), verifies its pinned SHA256, and extracts only the core and its GPL license. It cross-compiles the wrapper using `CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 GOAMD64=v1`, creates `dist/Retro VLESS.app`, and writes `dist/Retro-VLESS-Mac-Intel.zip` plus `dist/SHA256.txt`. Cached downloads are under `build/`.

The ZIP includes the app, README, GPL license, attribution, and wrapper sources/build script. It does not collect local settings. Upstream core source is linked below and in `SOURCES.txt`; it is not included in the ZIP.

## Tests

```sh
go test ./...
```

Optional native-core checks and a loopback-only Linux VLESS integration test:

```sh
SING_TEST_BINARY=/path/to/linux/sing-box XRAY_TEST_BINARY=/path/to/linux/xray SING_INTEGRATION=1 go test -v ./...
```

Use a Linux sing-box 1.3.6 binary for these tests. Xray is a test fixture only; it is not bundled or used by the app. Integration tests skip if their opt-in flag or core fixtures are absent. No provider credentials or external service are used by the loopback test. Some legacy Xray configuration-generation helpers remain solely for existing unit-test coverage; production connection setup uses sing-box.

## License and upstream attribution

Wrapper source is distributed under **GNU GPL v3 or later**; see `LICENSE`. Bundled sing-box is by SagerNet and contributors, **GPL-3.0-or-later**; its unmodified license is in the app's `Contents/Resources/sing-box-LICENSE.txt`.

- Official release: https://github.com/SagerNet/sing-box/releases/tag/v1.3.6
- Corresponding upstream source: https://github.com/SagerNet/sing-box/tree/v1.3.6
- Upstream source archive: https://github.com/SagerNet/sing-box/archive/refs/tags/v1.3.6.tar.gz

Retro VLESS is a separate wrapper project. Upstream names identify the bundled components, not authorship or endorsement of this wrapper.
