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

## Bearer JWT 认证（2026-09-30）

登录返回 access_token/token_type/expires_in/user，有效期 30 分钟；前端只保存于内存，刷新需重新登录。API 请求使用 `Authorization: Bearer <access_token>`，不再接受旧 Cookie 或 URL token；/api/me 保留但无定时轮询。下载通过带 Bearer 的 fetch 后生成 Blob，因此直接粘贴下载地址会返回 401。退出撤销当前 jti；刷新丢失但未退出的 token 到期前仍有效。

既有部署升级需在后端 `.env` 配置独立随机 `JWT_SECRET`（至少 32 字符），移除 SESSION_SECRET/COOKIE_SECURE；不要提交 .env。新建环境由 `npm run local:env` 自动生成。迁移版本 2 只新增 revoked_tokens，旧 sessions 表不再参与认证，账号和材料保留。Nginx 继续是唯一入口，页面壳可匿名获取，数据接口必须验证 JWT。

本节覆盖前述历史验收中的 Cookie/session 行为；第四课检索仍未实现。

JWT 验收：Go 单元测试/vet、前端构建、7 项 Playwright、事务回滚、JWT 撤销、停 API/停库恢复及 Compose down/up 数据保留均通过；两份当前变更 strict 校验通过。

## 第四课：可追溯检索

`docker compose up -d --build` 启动四个容器；只有 Nginx 的 `127.0.0.1:3000` 发布端口。Qdrant v1.15.5 使用独立持久卷，数据库自动增量迁移，原账号和材料保留。登录后可检索或“基于资料回答”；教师上传时可选三种切片方式，材料详情可显式重建。上传后稍候刷新列表，索引“已就绪”才可检索；失败可重建重试。

本机 `.env` 配置 EMBEDDING_BASE_URL / EMBEDDING_API_KEY / EMBEDDING_MODEL=course-embedding / EMBEDDING_DIM=2048；CHAT_MODEL=course-chat。CHAT_BASE_URL / CHAT_API_KEY 省略时沿用前者；禁止提交 `.env`。正文切片存 MySQL，向量和五个标识存 Qdrant。查询阈值 0.35、混合 RRF k=60；问答取前四片，无命中不调用对话服务。

验证：`npm run build`、`npm run test:e2e`（第四课测试会调用真实模型并上传合成材料）；Go 单元测试使用 `go test ./...`。截图位于忽略提交的 `evidence/week04-*.png`。故障时 keyword 仅依赖 MySQL；vector/hybrid 返回 503，不能把故障当成“没有资料”。

本轮验收：13 项端到端测试、Go 单元/MySQL 故障集成测试、构建及 OpenSpec strict 通过。`node scripts/verify-retrieval.mjs` 可复验 Qdrant 停机与恢复（短暂停止后自动恢复）。MySQL 集成测试需设置 `CAMPUS_TEST_DB=1` 及本机数据库环境；默认 `go test` 跳过该项，测试记录会自动清理。模型更换前需重建集合与全部材料索引，不能混用不同嵌入模型。

## 第五课：智能体、技能与流式会话

教师在“解题助手 → 教师：智能体与技能配置”维护本班智能体的角色提示词、启用状态及技能/材料绑定；学生选择启用智能体创建自己的会话。每轮只检索绑定的本班材料，引用出处来自 MySQL 切片；没有证据仍流式回答，但引用为空。只有选中“解题引导”技能时才施加不直接给最终答案的约束。第四课 `/api/ask` 的无依据固定提示保持原样。

浏览器打开 `http://127.0.0.1:3000/swagger/` 可查看和试调 OpenAPI；受保护接口用登录返回的 Bearer 令牌。前端只在内存保存令牌，刷新需重新登录。流式接口为 `POST /api/conversations/{id}/messages`，依次给出 meta、delta、done 或 error；流后错误的 HTTP 状态仍为 200，需检查事件。

接口验收：`docker compose --profile test run --rm --build apitest`。测试脚本位于 `apitest/cases/<URL>/<METHOD>.py`，会在合成班级中执行真实 MySQL 和模型检查，生成忽略 Git 的 `apitest/reports/report.html`；也可单独在测试容器中运行脚本。Go 集成测试在 Compose 网络中设置 `CAMPUS_TEST_DB=1`；前端执行 `npm run build` 与 `npm run test:e2e`；规约执行 `openspec validate add-guided-streaming-conversations --strict`。人工同伴复核需另行记录，自动化报告只代表程序检查。
