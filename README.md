# 小云朵

个人自用的本地游戏自动化原型，由 `gardend` 守护进程和内嵌 Web 控制台组成。

> 本项目仅供学习和本人授权账号的本地使用，不保证功能完整性、正确性或长期可用性。使用者应自行遵守相关服务条款、平台规则和当地法律法规。

## 安装

Linux / macOS：

```sh
curl -fsSL https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.sh | sh
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -Command "iwr https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.ps1 -UseB | iex"
```

也可以从 GitHub Release 下载对应平台的压缩包并运行其中的安装脚本。

## 启动

```sh
JWT_SECRET="$(openssl rand -hex 32)" \
ADMIN_PASSWORD="Use-A-Long-Local-Admin-Password-123!" \
gardend serve --listen 127.0.0.1:50051
```

打开 <http://127.0.0.1:50051>，使用管理员账号登录后添加游戏账号。默认管理员用户名为 `admin`。

目前仅支持 **iOS** 和 **Alipay**：iOS 使用游戏账号密码，Alipay 通过二维码自动完成授权。控制台按基础、花园、订单、公会、活动、仓库、统计和日志组织；读取状态通过一条 Protobuf WebSocket 推送，明确的账号与策略命令使用 Connect API。

公开兑换码中心位于 `/redeem`。管理员可订阅其他 MyGardenWorld 节点或自定义只读来源；节点订阅填写对方站点根地址（如 `https://gardend.example.com`），无需填写接口路径。

[查看社区兑换码的数据流与可信闭环](assets/redeem-exchange.svg)。

数据默认保存在系统用户配置目录下的 `mygardenworld/data`。事件与操作日志默认保留 7 天，可通过 `gardend serve --log-retention-days N` 调整；`0` 表示永久保留，`1` 表示保留 1 天。清理后 SQLite 会复用空闲页，但文件不会自动缩小；如需归还磁盘空间，先停止 `gardend`，再运行 `gardend compact-db --yes`。

如需重建本地数据：

```sh
gardend reset-data --yes
```

服务默认只监听回环地址。账号凭据和可恢复 Session 会在写入 SQLite 前使用本地密钥加密；备份时应同时保护 `garden.db` 和 `garden.db.key`。

## 从源码开发

需要系统 Go 1.27.0、Node.js 22、pnpm 10；重新生成协议还需要 Buf CLI。

```sh
make build
make test
make lint
make frontend:test
make frontend:lint
make frontend:build
```

`make check` 会执行完整质量门禁。调试游戏协议回包时使用 `make backend:debug`，普通启动不会写入 debug JSONL。

主要目录：

- `cmd/`：守护进程和协议辅助工具
- `internal/`：协议、状态、自动化、Runner、存储和 API
- `proto/`、`gen/`：Protobuf 源文件与生成代码
- `web/`：Next.js Web 控制台

协议行为以实际观测、`internal/babigame/doc.go`、Protobuf、代码和测试为准。开发约束见 [`AGENTS.md`](AGENTS.md)，第三方组件声明见 [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md)。
