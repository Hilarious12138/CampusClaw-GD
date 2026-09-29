## Why

第 3 课已完成登录、角色校验、班级隔离和材料入库。本次让教师与学生用自然语言在本班资料中定位片段，并提供可核对的出处；没有依据时明确返回未找到。

## What Changes

- 将材料正文按 auto（800 字/重叠 80 字）、custom、hierarchy 三种策略切片，保存文本、序号与字符区间到 MySQL `knowledge_chunks`。
- 调用服务端嵌入服务生成向量，写入 Qdrant；point ID 与切片 ID 相同，payload 仅存课件规定的五个关联标识，不存正文或摘录。
- 新增 keyword、vector、hybrid 检索，默认 hybrid；keyword 只查询 MySQL 全文索引，vector 使用余弦阈值 0.35，hybrid 用 RRF 融合两路名次。
- 检索班级只取自认证后的服务端会话 class_id，并复核成员关系，请求携带的班级编号不生效；教师、学生均可检索。
- 返回材料标题、切片序号、字符区间、原文摘录及材料打开入口。无候选时 hits=[]，提示“资料中未找到相关内容”；不编造出处。
- 增加 POST /api/ask：以最新一句做本班 hybrid 检索，最多取 4 片，有依据才调用对话模型生成带 [1] 引用的简短回答；无依据不调用生成模型，citations=[]。
- 教师可按本次请求的策略重建本班材料索引，先清理旧切片与旧向量；custom 预处理不改原文，偏移注明相对预处理后文本。
- 上传成功后触发切片和向量索引；启动时对种子与既有材料幂等补建索引，支持失败重试。新增 Qdrant 内部服务与持久卷，不发布宿主机端口。

## Capabilities

### New Capabilities

- `knowledge-retrieval`：本班材料的三策略切片、索引重建、三模式检索、简短问答及出处追溯。

### Modified Capabilities

无需重写 `auth-upload` 既有规约：上传原文的 201 仍代表原文件和正文已入 MySQL，不代表向量已就绪；后续索引状态属于新能力。既有登录、教师上传权限、详情及下载鉴权保持原契约；会话增量补 class_id、上传接受可选切分参数属于兼容扩展，具体设计见 D2/D5。

## Non-goals

不重做第 3 课功能；不实现流式输出、长对话产品、重排序、编排框架、跨班检索、公开向量库、多班级切换、额外文件格式或 OCR。本轮仅规划，不执行 Apply，不改代码、不安装依赖、不启动容器或调用付费模型。

## Impact / Scope Authority

后续实现涉及 Go 检索/索引模块、MySQL 迁移、Qdrant、前端检索入口及 Compose。React 18、TypeScript、Vite、Go net/http、MySQL 8.0、Nginx 基线不变。

已于 2026-09-29 对照用户指定的[课程网站](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/index.html)及 scope、rag、flow、chunk、class-scope、answer、think 七个章节修订。补入简短问答、三种切分策略、教师重建及诊断验收。网站提及 week04 原 spec，但未提供可直接取得的原文件链接；本次对齐的是完整公开课件，不冒称已读取该原文件。若后续原 spec 有差异再核对。

课程选定服务端兼容 OpenAI 的嵌入网关及独立对话网关；具体地址、模型名、维度和凭证仍待提供。参数未由课件限定的部分属于 design 中的本项目提案，不冒称课程硬性要求。
