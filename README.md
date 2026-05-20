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

`gardend` 是必须启动的本地服务进程；Web 控制台和 `gardenctl` 都只是操作入口，按你的习惯二选一或同时使用即可。

启动本地服务：

```sh
JWT_SECRET="$(openssl rand -hex 32)" ADMIN_PASSWORD="change-me-first" \
  gardend serve --data-dir ./data --listen 127.0.0.1:50051
```

打开 Web 控制台：

```text
http://127.0.0.1:50051
```

Web 控制台适合日常可视化管理账号、查看田地/库存/任务、启停自动化和调整策略。只用 Web 控制台也可以完成主要操作，不要求必须使用 `gardenctl`。

`gardenctl` 是可选的命令行客户端，适合脚本化、远程终端、查看 JSON 输出、订阅事件流，或在没有浏览器环境时操作同一个 `gardend` 服务。

登录本地控制面：

```sh
gardenctl auth login --username admin --password change-me-first
```

添加账号：

```sh
gardenctl account add main \
  --username "<account>" \
  --password "<password>" \
  --channel ios \
  --login
```

查看状态：

```sh
gardenctl --account main status
gardenctl --account main snapshot
gardenctl --account main watch
```

更新命令行程序：

```sh
gardenctl update
gardend update
```

更多用法请查看命令帮助：

```sh
gardend --help
gardend serve --help
gardenctl --help
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
