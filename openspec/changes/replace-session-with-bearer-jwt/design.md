## Decisions
1. Go net/http 保留，采用 golang-jwt/jwt/v5（成熟 JWT 库）固定 HS256；JWT_SECRET 至少 32 随机字符，仅后端环境变量。声明 iss=campusclaw-gd、aud=campusclaw-api、sub=user_id、jti=随机 ID、iat/nbf/exp，有效期 30 分钟。不把角色/班级作为 token 中的权威信息。
2. 仅接受 Authorization: Bearer；忽略 Cookie、URL token，401 带 WWW-Authenticate: Bearer。验证签名、iss/aud/exp/nbf/iat 和必需声明后，再查 users 和 revoked_tokens；权限/班级查询沿用现有数据库 JOIN，不信任请求角色。
3. 增量迁移版本 2 创建 revoked_tokens(jti,expires_at)，退出时持久化撤销并清理过期记录；旧 sessions 表保留但不读写，旧 Cookie 不再有效，登录响应清除旧 Cookie。服务不是完全无状态，保留撤销表用于立即退出失效。无 refresh token；重新登录获得独立 token，刷新丢失的旧 token 最多在到期前仍有效。
4. 前端用内存 ref 保存 token；请求 credentials=omit、自动附加 Bearer，401/到期/退出清空身份和材料。用认证代次与 AbortController 防止退出后的旧请求回填内容。没有 me 轮询；登录直接使用返回的 user。刷新显示登录页，静态页面可匿名下载但不含受保护数据。
5. Cookie CSRF token 机制移除，因为凭证不自动随浏览器请求发送；登录/写操作保留 Origin allowlist。下载走同源 fetch + Authorization，成功后创建短生命周期 Blob URL 并撤销，URL 中从不出现 JWT。
6. Nginx /_auth 子请求删除，页面入口只检查 /health；/api Authorization 显式转发。保持 api/mysql 不发布宿主端口。所有响应 no-store，JWT 不写日志。
## Verification
签名篡改、none/非 HS256、错误 issuer/audience、缺失或过期声明均 401；旧 Cookie/URL 不认证；真实师生登录、退出后复用失败、跨班拒绝、带 token 下载、刷新退出、无浏览器持久 token、无 me 轮询、服务故障清空材料。既有账号/文件在迁移前后保留。
