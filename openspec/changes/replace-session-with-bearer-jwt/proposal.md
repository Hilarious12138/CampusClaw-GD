## Why
用户已选择 Authorization: Bearer JWT，替换服务端 Cookie session 认证。
## What Changes
- 登录返回 access_token、token_type、expires_in 与用户/班级信息；token 只保存在前端内存，刷新需重新登录。
- API 固定验证 HS256、issuer、audience、有效期，再从数据库读取最新角色和班级。
- 下载通过 Bearer fetch 获取 Blob；不在 URL、Cookie 或浏览器持久存储中放 token。
- 退出记录 JWT jti 撤销信息，旧 token 立即失效；不再使用 sessions/CSRF。
- Nginx 仅健康检查页面入口，页面路由由前端判断身份，数据 API 强制认证；删除 me 定时轮询。
- 第四课检索规划改为可信 JWT 身份上下文，仍未执行检索 Apply。
## Capabilities
### New Capabilities
- bearer-auth：Bearer JWT 的签发、验证、撤销及前端生命周期。
### Modified Capabilities
旧 auth-upload 的 Cookie 会话设计由本变更覆盖，其角色、班级、上传/下载契约保留。
## Non-goals
不实现 refresh token、记住登录、多设备管理或第四课检索；不发布到 GitHub，除非用户另行要求。
