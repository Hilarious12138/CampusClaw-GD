## Why

第四课已有 Bearer 登录、本班检索和单次问答；第五课需要教师可配置的解题助手，让学生在个人会话中流式看到有真实出处的引导，并用接口文档和数据库断言验收。

## What Changes

- 教师维护多个本班智能体的角色提示词、启用状态及技能/材料绑定；预置数学、语文助手和“解题引导”技能。
- 每轮只从该智能体绑定且启用的技能中选择至多一条，只检索绑定的本班材料；引用来自 MySQL 切片。无引用仍可调用模型回答，但不编造出处。
- 学生与教师选择启用智能体创建个人会话，通过 Bearer POST/SSE 看增量输出、历史及引用。
- 提供与实现一致的 OpenAPI、同源 Swagger，以及按 URL/METHOD 分脚本的 apitest 和不入 Git 的 HTML 报告。
- 保留第四课检索、`/api/ask`、认证、上传和知识库索引行为。

## Capabilities

### New Capabilities
- `guided-conversations`：智能体、技能路由、材料绑定、个人流式会话、Swagger 和接口验收。

### Modified Capabilities
- MySQL 会话增加固定 agent_id；内部检索增加绑定材料范围。

## Non-goals

不重做登录/上传/切片，不做笔记、收藏、作业、任意代码工具、跨班共享或公网部署。开发模型偏好为 6.1sol；项目模型接口继续使用 course-embedding 与 course-chat。
