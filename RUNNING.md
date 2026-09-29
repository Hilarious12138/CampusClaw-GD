# CampusClaw-GD · 迭代 1

React 18 + TypeScript + Vite；Go `net/http`；MySQL 8.0；Nginx + Compose。仅实现登录、班级知识库与教师上传；支持 UTF-8 TXT/MD，单文件不超过 10 MiB。

## 启动

在项目目录运行（需要 Docker Desktop 已启动、Node 24）：

```sh
npm ci
npm run local:env
docker compose up -d --build
```

打开 http://127.0.0.1:3000 。`.env` 首次生成随机密码，已有文件不会覆盖。账号 `teacher_a`、`student_a`、`teacher_b`、`student_b` 的密码分别对应 `SEED_TEACHER_A_PASSWORD` 等变量。仅在本机查看该文件，不提交或发送密码。首次启动自动迁移并预置 A/B 两班和各自学习指南；以后重启不覆盖既有账号和材料。

若端口冲突，同时修改 `.env` 的 `WEB_PORT` 和 `APP_ORIGIN` 后重建服务。前端开发运行 `npm run dev`，访问 http://127.0.0.1:5173；Vite 仍经 Nginx 代理 `/api`，Go/MySQL 不开放宿主端口。

```sh
docker compose ps
curl http://127.0.0.1:3000/health
docker compose down
docker compose up -d
```

`down` 保留命名卷；不要加 `-v`，它会删除数据库。原文件与解析正文均在 MySQL 中，临时上传文件请求结束即清理，Nginx 的 `/uploads` 返回 404。

## 验证

```sh
npm run build
docker build -f deploy/api.Dockerfile --target test .
npm run test:e2e
OPENSPEC_TELEMETRY=0 openspec validate establish-campus-knowledge-access --strict
```

容器测试无需本机安装 Go。E2E 使用本机 Chrome 和 `.env` 演示账号，在数据库中新增名为「迭代验收.md」的测试材料。任务完成状态及验收范围以本次变更的 `tasks.md` 为准。

仅在本机演示环境可追加运行 `node scripts/verify-seed.mjs` 与 `node scripts/verify-runtime.mjs`：前者使用临时数据库，后者短暂停止 API/数据库以验证恢复，并保留应用数据。其他系统可通过 `DOCKER_BIN` 指定 Docker 命令路径。

## 后续开发

先读 `openspec/config.yaml` 与相关能力规约；概览列出范围、技术栈、接口/模型变化与验证方式，和用户敲定后实施。每次变更只维护 proposal、design、spec、tasks；完成后更新勾选，用户验收后归档到长期 specs。新想法先放 proposal 的 Non-goals，不增加重复文档。

## 本次验收（2026-09-23）

构建、Go 测试/静态检查、OpenSpec strict 和 7 项 Playwright 验收通过；种子缺参回滚与幂等、哈希存储、会话过期、数据库写入失败回滚、空列表、临时文件清理、停 API 清空页面、停库健康状态、Compose down/up 持久化均已实测。Vite 5173 与 Nginx 3000 的 `/api/me` 匿名 401 / 登录后 200 一致，`/uploads` 为 404。源码、产物及容器日志未发现实际密码或密钥。
