## 1. 规约与数据
- [x] 1.1 对照第五课课件确定智能体、技能、材料边界和技术栈；verify：proposal/design/spec 覆盖 Bearer、SSE、Swagger、apitest。
- [x] 1.2 增量迁移智能体、绑定和会话字段；verify：原数据保留，MySQL 集成测试可创建绑定与会话。

## 2. 业务接口
- [x] 2.1 教师智能体/技能配置与两班隔离；verify：学生写 403、外班/不存在同体 404、绑定只限本班。
- [x] 2.2 创建/继续个人会话；verify：agent_id 固定，停用仅阻新建，历史仅本人本班。
- [x] 2.3 技能路由和绑定材料检索；verify：词评分/平分可判定，MySQL/Qdrant/回表均限制材料，出处可追溯。
- [x] 2.4 真流式及故障；verify：meta/delta/done/error 顺序、无证据仍生成但无虚构引用、落库后 done、取消/并发正确。

## 3. 页面与契约
- [x] 3.1 教师配置和学生会话页面；verify：可维护智能体/技能/绑定，学生选择启用智能体、查看流式历史与引用。
- [x] 3.2 OpenAPI/Swagger；verify：实际路由/权限与文档一致，浏览器可试调，令牌不持久化。

## 4. 验收
- [x] 4.1 完成 URL/METHOD 接口测试；verify：一键 HTML 忽略提交，登录/检索/配置/会话/消息核对数据库。
- [x] 4.2 运行 Go、前端、浏览器与旧功能回归；verify：本机 Compose 真实模型流式输出及隔离场景通过。
- [x] 4.3 严格验证 OpenSpec 与精简运行说明；verify：openspec validate --strict 通过，文档和代码一致。
