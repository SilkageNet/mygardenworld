# 本地开发启动说明

## 当前运行状态

| 服务 | 地址 | 说明 |
|------|------|------|
| 后端 `gardend` | http://127.0.0.1:50051 | API + 内嵌控制台 |
| 前端 Next.js | http://127.0.0.1:3000 | 开发控制台 |

- **开发控制台**：http://127.0.0.1:3000
- **API / 内嵌控制台**：http://127.0.0.1:50051
- **登录**：`admin` / `change-me-first`

## 前置条件

- Go、pnpm 已安装
- 工作目录：`e:\work\mygardenworld`
- 首次前端需安装依赖：`pnpm --dir web install --frozen-lockfile`

## 后端

### 构建

```powershell
New-Item -ItemType Directory -Force -Path bin | Out-Null
go build -o bin\gardend.exe .\cmd\gardend
```

### 启动

```powershell
$env:JWT_SECRET = "local-dev-jwt-secret"
$env:ADMIN_PASSWORD = "change-me-first"
.\bin\gardend.exe serve `
  --data-dir data `
  --listen 127.0.0.1:50051 `
  --jwt-secret $env:JWT_SECRET `
  --admin-username admin `
  --admin-password $env:ADMIN_PASSWORD `
  --admin-email admin@localhost `
  --cors-origins "http://localhost:3000,http://127.0.0.1:3000" `
  --log-level info `
  --log-format text
```

成功日志示例：

```text
gardend listening addr=127.0.0.1:50051 data_dir=data
```

### Makefile 方式

```powershell
$env:JWT_SECRET = "local-dev-jwt-secret"
$env:ADMIN_PASSWORD = "change-me-first"
make backend
```

Debug（写出 WS/HTTP JSONL）：

```powershell
make backend:debug
```

## 前端

```powershell
$env:NEXT_PUBLIC_API_URL = "http://127.0.0.1:50051"
pnpm --dir web dev --hostname 127.0.0.1 --port 3000
```

或：

```powershell
make frontend
```

成功日志示例：

```text
Local:   http://127.0.0.1:3000
Ready
```

## 重启（PowerShell）

```powershell
# 停端口
Get-NetTCPConnection -LocalPort 50051,3000 -State Listen -ErrorAction SilentlyContinue |
  ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
Get-Process -Name gardend -ErrorAction SilentlyContinue |
  ForEach-Object { Stop-Process -Id $_.Id -Force -ErrorAction SilentlyContinue }

Start-Sleep -Seconds 2

# 重建并启动后端（新终端/后台）
go build -o bin\gardend.exe .\cmd\gardend
$env:JWT_SECRET = "local-dev-jwt-secret"
$env:ADMIN_PASSWORD = "change-me-first"
.\bin\gardend.exe serve `
  --data-dir data `
  --listen 127.0.0.1:50051 `
  --jwt-secret $env:JWT_SECRET `
  --admin-username admin `
  --admin-password $env:ADMIN_PASSWORD `
  --admin-email admin@localhost `
  --cors-origins "http://localhost:3000,http://127.0.0.1:3000" `
  --log-level info `
  --log-format text

# 启动前端（另一个终端）
$env:NEXT_PUBLIC_API_URL = "http://127.0.0.1:50051"
pnpm --dir web dev --hostname 127.0.0.1 --port 3000
```

## 默认开发参数

| 参数 | 值 |
|------|-----|
| `LISTEN` | `127.0.0.1:50051` |
| `FRONTEND` | `127.0.0.1:3000` |
| `DATA_DIR` | `data`（SQLite：`data/garden.db`） |
| `JWT_SECRET` | `local-dev-jwt-secret` |
| `ADMIN_USERNAME` | `admin` |
| `ADMIN_PASSWORD` | `change-me-first` |
| `CORS_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` |

## 说明

- 生产/安装版可只跑 `gardend`，Web 控制台内嵌在 `http://127.0.0.1:50051`。
- 开发时前后端分开：前端 `3000` 通过 `NEXT_PUBLIC_API_URL` 访问后端 `50051`。
- 普通 `backend` / `gardend serve` 不会写 debug JSONL；需要协议排查时用 `make backend:debug`（默认目录 `./debug`）。
- 更完整的项目说明见根目录 [README.md](../README.md)。
