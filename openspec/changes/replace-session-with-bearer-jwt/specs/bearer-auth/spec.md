## ADDED Requirements
### Requirement: Bearer JWT 登录
系统 SHALL 校验已有账号密码后返回 JWT 和用户信息，凭证只能通过 Authorization 请求头提交。
#### Scenario: 登录与携带 token
- **WHEN** 有效用户登录并以 Bearer 请求 /api/me
- **THEN** 返回该用户，登录响应有 access_token/token_type/expires_in/user，token 不保存在 Cookie 或持久浏览器存储。
#### Scenario: 无效凭证
- **WHEN** JWT 被篡改、过期、声明缺失、算法/issuer/audience 错误，或仅用 Cookie/URL token
- **THEN** 返回 401，不读取受保护材料。
### Requirement: 当前权限与撤销
系统 MUST 从数据库验证当前角色与成员关系，退出后撤销当前 jti。
#### Scenario: 退出及越权
- **WHEN** 已退出的 token 被重用，或学生上传、A 班访问 B 班材料
- **THEN** 分别返回 401、403、404，不能通过 token/request 自带角色获得权限。
### Requirement: 前端生命周期与文件
系统 SHALL 内存保存 token，刷新/到期/退出清空身份；下载携带 Bearer 且不把 token 放在 URL。
#### Scenario: 刷新及异步请求
- **WHEN** 页面刷新或退出时仍有材料请求在途
- **THEN** 回到登录页，旧请求不能回填受保护数据，无定时 me 请求。
#### Scenario: 下载
- **WHEN** 本班用户点击下载
- **THEN** 通过带 Bearer 的 fetch 获得文件；直接无凭证访问下载地址返回 401。
### Requirement: 兼容迁移
系统 SHALL 保留已有账号及材料，只替换认证机制。
#### Scenario: 数据迁移
- **WHEN** 从第 3 课数据库启动新版本
- **THEN** 迁移幂等、账号和材料保留，旧 session Cookie 不生效，第四课检索仍仅规划。
