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

已有账号可在账号详情中重新登录或更新凭据，无需删除重建。基础设置支持复制、粘贴完整 JSON 来导出、导入配置；导入校验后仍需手动保存，不改变账号启停状态。添加账号时，也可选择已有账号的已保存配置作为初始配置。

每位系统用户（包括 admin）只能查看和操作自己的游戏账号。顶部铃铛可配置个人通知，支持企业微信、钉钉、飞书群机器人和自定义 Webhook（可复制 JSON 示例及查看对接说明）；钉钉、飞书支持加签。默认关闭，一个渠道覆盖本人全部游戏账号，通知设置不随游戏配置复制。仅支持公网 HTTPS，推送请求保护、会话失效等重要状态，默认同类异常冷却 30 分钟；失败有限重试，自定义接收端可按 `id` 去重，投递记录保留 7 天。不推送无法归属到用户的全局错误。

公开兑换码中心位于 `/redeem`。管理员可订阅其他 MyGardenWorld 节点或自定义只读来源；节点订阅填写对方站点根地址（如 `https://gardend.example.com`），无需填写接口路径。

[查看社区兑换码的数据流与可信闭环](assets/redeem-exchange.svg)。

数据默认保存在系统用户配置目录下的 `mygardenworld/data`。事件与操作日志默认保留 7 天，可通过 `gardend serve --log-retention-days N` 调整；`0` 表示永久保留，`1` 表示保留 1 天。清理后 SQLite 会复用空闲页，但文件不会自动缩小；如需归还磁盘空间，先停止 `gardend`，再运行 `gardend compact-db --yes`。

需要多个独立实例时，为每个实例指定不同的 `--data-dir` 和 `--listen`，例如分别使用 `gardend serve --data-dir ./data-a --listen 127.0.0.1:50051` 与 `gardend serve --data-dir ./data-b --listen 127.0.0.1:50052`（各自配置启动所需的密钥和管理员密码）。目录分别保存数据库及加密密钥；不要让多个守护进程共用同一数据目录，也不要在不同实例中同时运行同一个游戏账号。当前仅支持本机 SQLite，不支持共享数据库的集群部署或 MySQL。

游戏请求默认按账号间隔 2 秒，同一 RPC 至少间隔 8 秒，同一商城的购买及珍珠雇佣至少间隔 30 秒（心跳保活除外；已有更长冷却仍有效）。运维可通过 `serve --game-request-interval 2s --game-repeat-interval 8s --game-purchase-interval 30s` 调整。该间隔同时覆盖自动操作、手动命令与操作内部连续请求，只是本地预防措施，不代表服务端限流阈值。

普通部署直接停止服务即可统一断开游戏连接。如需保留 Web 可访问、暂停全部游戏交互，可在服务所在机器使用同一版本的 `gardend` 和相同的 `--data-dir`：

```sh
gardend maintenance on          # 等待守护进程确认停止在途请求与连接
gardend maintenance status      # 查看持久化请求及确认状态，不是服务健康检查
gardend maintenance off         # 解除维护，账号由各用户手动连接
# 或显式恢复当前仍启用自动化、且所属用户有效的账号：
gardend maintenance off --resume-enabled
```

维护状态跨重启保留，不修改任何用户策略；控制命令要求本机数据库访问权，Web 管理员没有跨用户账号权限。`--wait 0` 仅保存待处理请求，适用于停机时预设维护状态，不能视为维护完成。退出维护后再次正常重启服务，仍按用户当前的自动化配置恢复账号。数据库离线压缩/备份仍须停止守护进程，维护开关不能代替停机。

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
