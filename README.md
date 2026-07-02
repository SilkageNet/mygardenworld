# mygardenworld

个人自用的本地自动化工具原型。代码主要来自 vibe coding / AI 辅助学习与实验，不保证功能完整性、正确性或长期可用性。

## 免责声明

本项目仅供个人学习和本地自用，我从未通过本工具做任何盈利。请仅在你本人拥有或被明确授权管理的账号上使用，并自行确认符合相关服务条款、平台规则和当地法律法规。如果本仓库存在任何违规或不适宜公开的内容，请通过 GitHub 联系我，我会立刻删除本仓库或相关内容。

## 安装

Linux/macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.sh | sh
```

Windows PowerShell:

```powershell
powershell -ExecutionPolicy Bypass -Command "iwr https://raw.githubusercontent.com/SilkageNet/mygardenworld/main/scripts/install.ps1 -UseB | iex"
```

或者从 Release 下载对应系统的压缩包，解压后运行其中的 `install.sh` 或 `install.ps1`。

## 使用

`gardend` 是必须启动的本地服务进程，并内嵌 Web 控制台。日常管理都通过浏览器完成。

启动本地服务：

```sh
JWT_SECRET="$(openssl rand -hex 32)" ADMIN_PASSWORD="change-me-first" \
  gardend serve --data-dir ./data --listen 127.0.0.1:50051
```

本地数据默认由 `gardend serve --data-dir` 决定；不传时使用系统用户配置目录下的 `mygardenworld/data`，SQLite 文件为 `garden.db`。一键安装只安装 `gardend` 二进制，不会额外切换数据目录。

需要排查协议回包时，请用源码目录里的 debug 启动目标，而不是普通 `backend`：

```sh
make backend:debug
# 或
task backend:debug
```

`backend:debug` 会自动传入 `--debug-dir`，默认写到 `./debug/<账号名>_debug.jsonl`。如果直接运行 `gardend serve` 或 `make backend`，除非手动加 `--debug-dir`，否则不会生成 WS/HTTP debug JSONL。

重置本地数据：

```sh
gardend reset-data --data-dir ./data --yes
```

打开 Web 控制台：

```text
http://127.0.0.1:50051
```

Web 控制台适合日常可视化管理账号、查看田地/库存/任务、启停自动化和调整策略。首次启动后使用启动参数里的 `--admin-username` 和 `--admin-password` 登录，然后在页面里添加游戏账号。

自动化策略按 **基础、种植、订单、公会、活动** 五个业务域组织；账号会话与内部错误分别进入 **account** 和 **system** 日志分类。策略、执行计划、运行状态和日志过滤使用同一套分类。

更新本地程序：

```sh
gardend update
```

更多用法请查看命令帮助：

```sh
gardend --help
gardend serve --help
```

## 从源码构建

```sh
make build
make test
make frontend
```

`make frontend`、`make frontend:build`、`make frontend:lint` 会先执行 `pnpm --dir web install --frozen-lockfile`。使用 Taskfile 时，对应的 `task frontend`、`task frontend:build`、`task frontend:lint` 也会先跑 `frontend:deps`。

开发模式下前端和后端可以分开启动；Release 二进制会内嵌已构建的 Web 控制台。

## 发版

推送符合 `v*` 的 tag 会触发 GitHub Actions 自动构建 Release：

```sh
git tag v0.1.0
git push origin v0.1.0
```
