## Context

迭代 1 已获用户授权；按最新要求改为 React 18、Go net/http、MySQL 8.0、Nginx + Compose，替换未交付的 Node/PostgreSQL 实现。验收见 specs/auth-upload/spec.md。

## Goals / Non-Goals

**Goals:** 登录、角色权限、班级数据隔离、教师上传入库、预置样本、单一同源入口和健康检查。
**Non-Goals:** Web 框架、前端直连 Go/数据库、检索问答、AI 对话、作业、注册与找回密码。

## Decisions

### D1 技术栈与单一同源链路

React 18.3.1 + TypeScript + Vite；Go 标准库 net/http/ServeMux + database/sql，不引入 Web 框架；仅使用 MySQL 驱动与 x/crypto Argon2id 依赖。MySQL 固定 8.0 系列。

生产：浏览器→Nginx→Go→MySQL；Nginx 提供构建后的静态资源和同源 /api 反向代理，保留完整 /api 路径。开发：浏览器→Vite→Nginx→Go→MySQL；Vite 仅把相同 /api、/health 路径代理到 Nginx，前端源码不含 Go 地址或数据库配置。

Compose 仅 Nginx 映射宿主端口，默认 127.0.0.1:3000，可用 WEB_PORT 调整；api 与 db 不暴露宿主端口。开发来源 http://127.0.0.1:5173 由服务端允许列表显式配置，生产来源来自 APP_ORIGIN。

### D2 会话与密码

MySQL 持久化服务端会话；Cookie 只带不透明随机令牌，HttpOnly、SameSite=Lax，生产 HTTPS 启用 Secure，绝对有效期 8 小时。登录轮换并撤销旧会话，退出注销服务端会话。角色和班级来自数据库，不信任客户端字段。

Argon2id 使用独立随机盐，m=19 MiB、t=2、p=1，按 PHC 格式保存；验证时限制参数并恒定时间比较。会话密钥、数据库密码、种子初始密码只在服务端环境变量中；无默认明文口令、不输出到日志。登录校验 Origin，写入需 Origin + 会话 CSRF token。

### D3 查询层班级隔离

所有材料查询必须把当前 user_id、请求 class_id 与成员关系 JOIN；详情/下载进一步匹配材料真实 class_id 和 material_id。学生上传在解析前 403，教师跨班或跨班读取返回 404，无会话返回 401。写入事务在提交前锁定并再次核对教师角色和成员关系，防止上传期间权限变化。

### D4 数据模型

| 表 | 字段/约束 |
| --- | --- |
| users | id、username 唯一、display_name、password_hash、role=teacher/student |
| classes / class_memberships | 班级 id/code/name；user_id/class_id 组合唯一及外键 |
| sessions | 令牌 HMAC、user_id、csrf_token、expires_at |
| materials | id、class_id、uploader_id、filename、type、size_bytes、created_at |
| material_contents | material_id 主外键、body LONGTEXT、original LONGBLOB |
| schema_versions / seed_versions | 迁移/种子版本，重复启动不覆盖数据 |

使用 InnoDB、utf8mb4 和参数化 SQL。首次迁移采用幂等 DDL 与版本记录（MySQL DDL 隐式提交，不宣称事务回滚）；种子及上传使用完整事务。

### D5 上传入库数据流

认证→教师角色→班级成员→CSRF→限制请求体/单文件→解析 UTF-8 TXT/MD（≤10 MiB）→非空正文→事务写入元信息、正文、原始字节→提交后 201→本班列表查库。Markdown 显示为文本，不执行 HTML。

文件只在 Go 私有临时目录解析，所有退出路径清理；无可用正文 422、格式/编码不支持 415、超限 413、数据库故障 503。解析最长 10 秒，拒绝多文件或额外字段；失败不得留下部分入库结果。

### D6 预置与运行健康

A/B 两班、teacher_a/student_a/teacher_b/student_b 四个账号、每班一份合成教学材料及原文。四个初始密码通过 SEED_*_PASSWORD 输入并哈希；缺失则初始化失败；种子版本阻止重复导入、覆盖修改或恢复删除。

Compose 服务为 nginx、api、db。db 使用命名卷，健康检查使用环境中的数据库凭证验证连接；api 等 db healthy 后迁移、seed 再监听；api 用自身 healthcheck，Nginx 经 /health 验证完整链路。探测 interval=10s、timeout=5s、retries=3、start_period=30s。

/health 匿名、no-store；Go 在总计 2 秒内查询数据库及必要表/种子版本，就绪为 200 {"status":"ok"}，故障为 503 {"status":"unavailable"}。停止 api 后 Nginx 将错误转为 503；页面入口用内部健康子请求阻止继续提供正常页面，已打开前端下一次失败请求清空材料；不做离线缓存。Compose down/up 保留数据卷，日常不使用 down -v。

### D7 材料文件访问

原始内容在 MySQL，与正文原子提交；Go 临时目录不共享给 Nginx。Nginx 明确拒绝 /uploads/，不配置该路径的 alias/static。GET /api/classes/:classId/materials/:id/file 先做与详情相同的成员 JOIN，再以 attachment、nosniff 返回原字节，不产生公开文件链接。

### 接口一览

| 接口 | 结果 |
| --- | --- |
| POST /api/auth/login | 成功建立会话，凭证错误 401 |
| POST /api/auth/logout | 校验 CSRF 后注销会话 |
| GET /api/me | 身份、角色、本班列表与 CSRF token；无会话 401 |
| GET /api/classes/:classId/materials | 本班查库列表；无记录为空，非成员 404 |
| GET /api/classes/:classId/materials/:id | 已授权正文和元信息；跨班 404 |
| GET /api/classes/:classId/materials/:id/file | 已授权原文件附件；跨班 404 |
| POST /api/classes/:classId/materials | 本班教师上传；201/401/403/404/413/415/422/503 |
| GET /health | 由 Nginx 代理至 Go，200 或 503，不跳登录 |

## Risks / Trade-offs

- 文件入数据库增加体积 → 首期限定 10 MiB；后续更换私有文件存储不改变授权接口。
- MySQL 不支持回滚 DDL → 幂等迁移逐条执行，成功才记版本；数据写入仍使用事务。
- api 重建可能换地址 → Nginx 使用 Docker DNS 动态解析服务名，不依赖旧容器 IP。

## Verification

对照用户三个核对项分别检查规约、源码与运行：扫描前端连接和 Nginx alias；比较开发/Compose 的 /api/me；停 api 检查 503 与无材料数据；猜测 /uploads 路径验证 404；测试本班/跨班授权下载。
