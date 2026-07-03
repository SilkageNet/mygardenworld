# mygardenworld Web

本目录是 `gardend` 内嵌 Web 控制台的前端源码。

## 开发

安装依赖：

```bash
pnpm install --frozen-lockfile
```

启动开发服务：

```bash
pnpm dev
```

默认开发入口为 `localhost:3000`。生产发布时由仓库根目录的构建流程生成静态资源，并内嵌到 `gardend`。

## 常用命令

```bash
pnpm lint
pnpm build
```

根目录的 `make frontend`、`make frontend:build`、`make frontend:lint` 会自动进入本目录执行对应流程。
