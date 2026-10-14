## 1. 实现
- [x] 1.1 JWT 签发/验证与配置迁移；verify：固定算法、完整声明、错误 token 单元测试通过。
- [x] 1.2 增量撤销表及权限查询；verify：旧数据保留、退出复用 401、角色及跨班拒绝。
- [x] 1.3 前端内存 token、下载与 Nginx；verify：刷新需登录、无 Cookie/持久存储/轮询、退出后旧请求不回填。
## 2. 验证与同步
- [x] 2.1 更新接口/浏览器/运行脚本测试并验证构建与 Compose；verify：登录上传下载、篡改/过期/退出、数据持久性通过。
- [x] 2.2 更新第四课认证上下文、README/RUNNING/config；verify：OpenSpec strict 通过，检索 tasks 保持未勾选。
