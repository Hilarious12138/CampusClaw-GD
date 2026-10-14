## Context / 技术栈

增量扩展 React 18 + TypeScript + Vite、Go net/http、MySQL 8、Qdrant、Nginx + Compose。浏览器只访问同源 Nginx /api；数据库、向量库和模型密钥仅由 Go 使用。嵌入 course-embedding 2048 维，对话 course-chat。课件旧示例中的 Cookie/localStorage 与本项目 Bearer 内存令牌不一致，以既有认证规约为准。

## Decisions

### D1 身份与隔离

Go 验 JWT，再查数据库用户、角色及唯一成员班级。客户端的 class_id、角色、历史和消息中的 agent_id 均无效。配置、技能、材料按 class_id 限制；会话再按 user_id 限制。外班/同班他人/不存在资源统一 404 同体，未登录 401，学生写 403。绑定时在事务中确认技能和材料属于本班；检索的 MySQL 查询、Qdrant filter 与回表均限制绑定材料。

### D2 智能体与技能

tutor_agents：id、class_id、name（1～64 字，班内唯一）、prompt（最多 2000 字，可空回退）、enabled。tutor_agent_skills / tutor_agent_materials 为关联表，单类最多绑定 100 项。教师 GET/POST /api/agents、GET/PUT /api/agents/{id}；学生仅见启用智能体。停用阻止新会话，已有会话仍继续。

skills 存源文本及 name、trigger、subject、description、enabled。名称最多 32 字，触发词最多 120，主题最多 32，正文最多 2000，源文本最多 8000。头部支持 名称/适用条件/资料/工具/启用；资料和工具可选，工具仅 knowledge_search；忽略头部注释，拒绝未知/重复键。教师可在页面编写或导入文本文件；GET /api/skills/{id} 允许本班师生读，外班与不存在同体 404。按中英文逗号、顿号、分号或竖线拆触发词，问题包含则计分；只在绑定且启用技能中选最高分，平分取最小 ID，零分不选。仅选中的技能名称含“解题引导”时要求不直接给最终答案。

### D3 会话与消息

conversations 增 agent_id（创建必填，之后不变），旧 skill_id 可空仅作兼容。会话归创建者与班级，标题取首问 24 字。conversation_messages 存 turn_id、role、content、status、citations JSON、config_snapshot JSON；唯一(turn_id,role)。短事务先写问题与 generating 占位，再检索及生成。完成落库后发 done；断连/失败保留 cancelled/failed 草稿，历史生成排除草稿。连接锁确保同会话并发返回 409，遗留 generating 可恢复。

### D4 一轮回答

读取智能体与绑定技能 → 路由至多一条 → 最新问题 hybrid 检索绑定的本班材料 top4 → 去空正文及标题不含所选技能主题的切片 → 顺序编号引用。切片正文与出处从 MySQL 回取，向量库只存向量和标识。系统消息放角色提示词、选中技能说明及边界；证据放用户消息，每片最多 800 字。历史由服务端取最近六条有效消息，每条最多 500 字，含当前问题。

无证据仍调用对话模型，citations=[]，流式输出中的 [数字] 标号跨 token 清除；第四课 /api/ask 的固定“资料中未找到相关内容”行为保留。有证据时出处通过 meta/数据库展示，正文可不带内联编号，但凡出现编号必须有效，否则发 error 且不发 done。

### D5 SSE、代理和页面

浏览器 fetch POST 带 Bearer，增量解析 UTF-8 SSE；上游 stream=true 并逐片 Flush。模型前发 meta{status:"ok",agent_id,skill|null,citations}，随后多次 delta{text}，数据库完成提交后 done，流后失败 error（HTTP 仍 200）。取消中断上游，详情可查状态。Go 总期限 120 秒且不晚于 JWT 到期、上游空闲 30 秒；Nginx 单独流路由关闭 buffering/cache/gzip。教师页面编辑智能体、技能和绑定，学生选择启用智能体并查看历史/引用。

### D6 文档与验收

/openapi.json 描述真实路由、入参、响应、错误、Bearer 和 SSE；/swagger/ 用镜像内静态资源同源试调，不持久化令牌。apitest/cases/<URL>/<METHOD>.py 每接口一脚本，run_all.py 对照 OpenAPI 方法集合、用合成两班夹具并生成忽略 Git 的 HTML。断言 API 与 MySQL 行、归属、状态和引用一致；报告脱敏，测试容器内部访问 DB/Qdrant。人工同伴复核需实际复核者记录，自动测试不冒充人工复核。

## Verification

执行 Go 单元及数据库集成测试、前端构建、浏览器端到端、接口脚本、严格 OpenSpec 校验。只对实际验证过的任务勾选。
