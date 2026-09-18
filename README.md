<div align="center">

<img src="web/public/brand/cloud-logo.svg" width="88" height="88" alt="Cloud logo" />

# 小云朵

A small cloud for your garden. A self-hosted automation prototype with a built-in Web console.

[![Release](https://img.shields.io/github/v/release/SilkageNet/mygardenworld?style=flat-square&color=0ea5e9)](https://github.com/SilkageNet/mygardenworld/releases/latest)
[![CI](https://github.com/SilkageNet/mygardenworld/actions/workflows/ci.yml/badge.svg)](https://github.com/SilkageNet/mygardenworld/actions/workflows/ci.yml)
[![Stars](https://img.shields.io/github/stars/SilkageNet/mygardenworld?style=flat-square&color=eab308)](https://github.com/SilkageNet/mygardenworld/stargazers)

[Download](https://github.com/SilkageNet/mygardenworld/releases/latest) · [Quick start](#quick-start) · [Community](#community)

</div>

> [!WARNING]
> **Unofficial, experimental, and used entirely at your own risk.**
>
> This project is not affiliated with or endorsed by the game or its platforms. It is intended for learning and personal use with accounts you own or are authorized to operate. Automation may violate platform rules and lead to account restrictions, suspension, or loss of game progress and resources.
>
> No guarantees are made about safety, correctness, or continued availability. You are responsible for your use and for complying with applicable terms, rules, and laws. **Do not use it if you cannot accept these risks.**

## At a glance

- **One daemon, one dashboard.** Manage automation, account status, and logs from your browser.
- **Two supported channels.** iOS account login and Alipay QR authorization.
- **Your accounts stay yours.** Per-user account isolation, reusable JSON policies, and personal notifications.

## Quick start

Download a [release](https://github.com/SilkageNet/mygardenworld/releases/latest) for Linux, macOS, or Windows, or use an installer:

**Linux / macOS**

```sh
curl -fsSL https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.sh | sh
```

**Windows PowerShell**

```powershell
powershell -ExecutionPolicy Bypass -Command "iwr https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.ps1 -UseB | iex"
```

Open a new terminal after installation. Start the server with a strong admin password of your own:

```sh
JWT_SECRET="$(openssl rand -hex 32)" \
ADMIN_PASSWORD="Replace-With-Your-Own-Strong-Password" \
gardend serve --listen 127.0.0.1:50051
```

<details>
<summary>Starting on Windows PowerShell</summary>

```powershell
$bytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
$rng.Dispose()
$env:JWT_SECRET = [Convert]::ToBase64String($bytes)
$env:ADMIN_PASSWORD = "Replace-With-Your-Own-Strong-Password"
gardend serve --listen 127.0.0.1:50051
```

</details>

Open **[localhost:50051](http://127.0.0.1:50051)**, sign in as `admin`, and add your game account. Review its settings before enabling automation.

> [!TIP]
> Keep the default loopback binding for local use. Back up both `garden.db` and `garden.db.key` together after stopping the daemon, and keep them private. Use `gardend serve --help` for data-directory and other options.

## Community

If this project is useful to you, **leave a star** — it helps others discover it.

Questions, ideas, and thoughtful feedback are welcome in [Issues](https://github.com/SilkageNet/mygardenworld/issues). Please search existing threads first, and remove credentials, tokens, and personal information from logs or screenshots before sharing. Small, focused pull requests are welcome too.

## Development

Use **Go 1.27.0**, **Node.js 22**, and **pnpm 10**. Start with `pnpm --dir web install --frozen-lockfile`, then run `make check` for the quality checks.

See [AGENTS.md](AGENTS.md) for contributor guidance and [third-party notices](THIRD_PARTY_NOTICES.md) for dependency acknowledgments.
