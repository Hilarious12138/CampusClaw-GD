## Purpose

在第 3 课已有系统中检索本班教学材料，以真实切片为唯一出处。本文件已对照用户指定第 4 课公开课件全部章节修订（2026-09-29），包含简短问答；出处链接见 design 的课程对照。网站引用的 week04 原 spec 未取得，不宣称已读取原件；用户于 2026-09-30 授权实施。

## ADDED Requirements

### Requirement: JWT 身份与班级边界
系统 MUST 复用现有 Bearer JWT 认证；教师与学生均可检索本人班级。检索班级仅从JWT 验证后、由数据库成员关系确定的服务端 class_id 上下文取得，认证模块复核数据库成员关系，客户端班级编号不得作为依据。

#### Scenario: 本班两类用户检索
- **WHEN** 属于 A 班的教师或学生使用有效 JWT 提交检索
- **THEN** 仅返回其有权读取的 A 班材料切片，上传仍仅限本班教师

#### Scenario: 客户端伪造班级
- **WHEN** A 班用户在 body、query 或 header 中携带 B 班 class_id/classId
- **THEN** 这些字段不改变检索范围，不返回 B 班切片、标题或摘录

#### Scenario: 未登录或无法唯一确定班级
- **WHEN** 请求没有有效 JWT
- **THEN** 返回 401，不查询切片或调用嵌入服务
- **AND** JWT 对应用户没有班级时返回 403，关联多个班级而未建立唯一服务端班级上下文时返回 409，不采用请求中的班级编号

### Requirement: 三种切分策略及原文保留
系统 SHALL 在上传和重建时支持 auto、custom、hierarchy 三种策略；切分模块只处理文本，不调用嵌入服务或写入 Qdrant。原文件和持久化正文不因预处理而改变。

#### Scenario: 默认自动窗口
- **WHEN** 未指定策略或采用 auto，包括启动时种子索引补建
- **THEN** 最大 800 个 Unicode 码点，重叠 80 个，优先在空行、换行、句号处断开，无合适边界时强制截断；保留非空尾片
- **AND** auto 忽略请求另填的长度、重叠与预处理参数，不截断多字节字符或陷入重复窗口

#### Scenario: 自定义参数与边界
- **WHEN** custom 选择换行/空行/句号，最大长度为 100～2000 整数、重叠比例为 0～50%
- **THEN** 按所选分隔符切片，无断点处按最大长度截断，100/2000 和 0/50% 边界可接受
- **AND** custom 非法长度、比例、分隔符或未知策略返回 400，不修改原材料和现有索引

#### Scenario: Markdown 分章
- **WHEN** hierarchy 处理包含 #、##、### 标题的材料
- **THEN** 按标题层级分章且保留标题，过长章按 auto 再切，无标题按 auto 处理

#### Scenario: 自定义预处理
- **WHEN** custom 启用移除 URL/邮箱或折叠连续空白
- **THEN** 仅待切分/嵌入文本发生变化，原文件与 MySQL 原文不变
- **AND** 折叠空白后换行消失时，不假装仍能在换行处分隔

### Requirement: 可解释的切片位置
系统 SHALL 保存 chunk_index、字符区间及 offset_basis；序号从 1 起，位置按 Unicode 码点从 0 起、右端不包含，不把预处理后偏移误称为原文偏移。

#### Scenario: 无预处理的出处
- **WHEN** auto/hierarchy 或未启用预处理的 custom 产生切片
- **THEN** chunk_text 等于原文对应区间，中文/emoji/尾片可逐字符还原

#### Scenario: 有预处理的出处
- **WHEN** custom 删除链接或折叠空白后产生切片
- **THEN** 偏移相对于持久化的预处理文本，offset_basis=processed；界面明确标注，不能将该偏移直接用于原文件高亮
- **AND** 可查看完整切片及打开未改写的原材料

### Requirement: MySQL 正文与 Qdrant 向量分离
系统 MUST 将切片正文存于 MySQL knowledge_chunks，将向量存于 Qdrant；point ID 与 knowledge_chunks.id 相同，向量 payload 只含必要标识。

#### Scenario: 核对存储边界
- **WHEN** 某材料完成索引
- **THEN** MySQL 有材料/知识条目关联、切片序号、字符区间与 chunk_text；Qdrant 集合为 campusclaw_chunks、度量 Cosine，payload 仅 class_id、material_id、knowledge_entry_id、chunk_id、chunk_index
- **AND** 向量主键 = knowledge_chunks.id = payload.chunk_id；Qdrant 不保存正文、标题、摘录或原文件，检索返回的摘录来自 MySQL

### Requirement: 上传后索引及幂等补建
系统 SHALL 在既有上传成功后执行切片和嵌入，记录切片 index_status=pending/ready/failed；种子及既有材料启动补建采用默认窗口策略，不能重做登录、种子账号或既有上传鉴权。

#### Scenario: 上传到可检索
- **WHEN** 教师上传成功并完成切片与嵌入
- **THEN** 保留既有上传成功契约，界面区分待索引与可检索，当前版本切片及向量可被检索

#### Scenario: 嵌入失败及重试
- **WHEN** 嵌入、Qdrant 写入或进程执行中途失败
- **THEN** 已入库材料仍可按原权限读取，切片标 failed/待重试、不虚报 ready；嵌入失败的切片没有对应向量，部分写入故障清理残留点且回表拒绝 failed
- **AND** 重启或重试复用已存切片 ID，不生成重复切片或重复 point

#### Scenario: 既有材料与重复启动
- **WHEN** 首次部署检索能力或再次启动服务
- **THEN** 种子和已有材料的缺失索引被补建，已完成的相同版本不重复创建，不覆盖账号、密码或原文

### Requirement: Keyword 仅用全文索引
系统 SHALL 提供 keyword 模式，只查询 MySQL 全文索引，采用 FULLTEXT(chunk_text) WITH PARSER ngram（token 长度 2），对本班当前版本且 index_status=ready 的切片按全文相关度排序，不调用嵌入服务或 Qdrant。

#### Scenario: 原文词命中
- **WHEN** 本班原文包含有效查询词，用户选择 keyword
- **THEN** 返回全文匹配且得分大于 0 的候选，调用记录中没有嵌入服务、对话网关或 Qdrant 请求

#### Scenario: 无有效匹配
- **WHEN** 查询词不匹配本班切片，或输入仅含无有效检索词的标点
- **THEN** 返回 200、hits=[] 和“资料中未找到相关内容”，不返回无匹配切片凑数

### Requirement: Vector 语义检索与阈值
系统 SHALL 用与材料相同模型的嵌入检索 Qdrant Cosine 向量，搜索时强制认证上下文班级过滤，只保留余弦分数不低于 0.35 的有效候选。

#### Scenario: 语义相近的改写
- **WHEN** 验收问句与本班材料表达不同但语义对应，使用最终确定的真实嵌入模型
- **THEN** 返回相关切片及可核对出处，不能用伪造向量或硬编码结果通过验收

#### Scenario: 阈值边界
- **WHEN** 候选余弦分数分别为 0.349、0.350 和 0.351
- **THEN** 0.349 被剔除，后两者通过分数门槛，但仍须通过班级和版本校验；不得降低阈值补齐数量

### Requirement: Hybrid 默认 RRF 融合
系统 SHALL 默认采用 hybrid，分别执行 keyword 与 vector 的过滤及排序，再以 RRF 融合名次，不能直接相加两种原始分数。

#### Scenario: 默认模式与双路贡献
- **WHEN** 请求省略 mode
- **THEN** 执行 hybrid；RRF 使用 k=60、从 1 开始的名次，两路出现的相同 chunk ID 合并并累计贡献

#### Scenario: 可判定融合排序
- **WHEN** keyword 次序为 A、B，vector 次序为 B、C，且三者都通过各自过滤
- **THEN** B 得分为 1/62+1/61，A 为 1/61，C 为 1/62，最终次序为 B、A、C
- **AND** 两路都已过滤掉的切片不得在融合阶段重新出现

### Requirement: 回表校验与原文追溯
系统 MUST 对向量命中回 MySQL 读取并校验当前班级、材料、切片与索引版本；每条 hit 至少包含材料标题、切片 ID/序号、字符区间、原文摘录及材料打开入口。

#### Scenario: 打开出处
- **WHEN** 用户点击某条结果的材料入口
- **THEN** 经过既有材料鉴权打开对应原文，按 offset_basis 定位该切片区间；摘录是 MySQL chunk_text 的真实子串，完整切片可查看；若偏移基于预处理文本则明确标注并在对应文本中定位，不对原文错误高亮

#### Scenario: 脏向量与越权切片
- **WHEN** Qdrant 命中已删除、旧版本、未 ready 或实际属于 B 班的切片 ID，而当前身份属于 A 班
- **THEN** 回表时丢弃这些候选，不返回其标题或正文；直接访问越权切片接口返回 404

### Requirement: 无依据不生成结果
系统 MUST 在所选模式过滤后无候选时返回 hits=[] 与“资料中未找到相关内容”；检索接口不调用生成模型、不虚构引用；问答是否生成由命中证据决定，不为达到 top_k 而补入无依据切片。

#### Scenario: 本班没有相关内容
- **WHEN** 本班索引就绪且问题与验收语料无关，或相关内容仅存在于另一班
- **THEN** 对所测模式返回 200、hits=[] 和规定提示，不显示虚构材料或切片

#### Scenario: 候选数量不足
- **WHEN** top_k=5 而过滤后仅有两个候选
- **THEN** 只返回两个结果，序号与区间均可在 offset_basis 指定的原文或预处理文本验证

### Requirement: 参数、索引状态与故障区分
系统 SHALL 校验 query/mode/top_k，区分无命中、索引未完成和依赖故障；不得将依赖失败伪装为资料无依据。

#### Scenario: 非法参数
- **WHEN** query 为空白或超过 500 个码点、mode 不合法、top_k 非整数或超出 1～20
- **THEN** 返回 400，不发起嵌入或向量检索

#### Scenario: 向量依赖不可用
- **WHEN** 嵌入或 Qdrant 超时/不可用，用户选择 vector 或 hybrid
- **THEN** 返回 503 和服务不可用提示，不静默降级、不返回“未找到”冒充成功
- **AND** keyword 在 MySQL 正常时仍能工作

#### Scenario: 索引尚未完整
- **WHEN** 本班有 pending/indexing/failed 材料
- **THEN** 响应包含未完成/失败数量，页面明确显示索引状态；无候选时保留规定无命中文案，同时提示并非全部资料可检索

### Requirement: 内部部署与凭证边界
系统 MUST 沿用同源 /api 入口；Qdrant 仅对内部服务开放并持久化，嵌入配置及密钥只存在服务端。

#### Scenario: 启动与重建
- **WHEN** 配置确认后的模型服务并通过 Compose 启动或保留数据卷重建
- **THEN** 已有账号、材料、切片与向量保留，缺失索引可恢复；仅 Nginx 发布宿主端口，前端产物无模型密钥或 Qdrant 连接凭证

#### Scenario: 检索依赖故障不阻断既有系统
- **WHEN** Qdrant 或嵌入服务不可用而 MySQL 正常
- **THEN** 登录、既有材料读取与 keyword 保持可用；已 ready 切片的 keyword 有效；Qdrant 有独立健康检查，既有 /health 不因模型故障阻断整个页面

### Requirement: 本班教师显式重建索引
系统 SHALL 允许本班教师按本次请求策略重建，先清理旧切片与旧向量，再写入新切片/向量；保留原文，不接受学生重建。

#### Scenario: 切换策略后重建
- **WHEN** 教师对同一材料依次采用三种策略重建
- **THEN** 新索引对应本次参数，旧 point ID 与旧 chunk ID 被清除，不混用旧策略结果，原文保持一致
- **AND** 仅修改参数而未发起重建时，已有切片不改变

#### Scenario: 重建权限与失败
- **WHEN** 学生请求重建、教师指定他班材料或旧向量清理失败
- **THEN** 分别返回 403、404 或记录明确任务失败；不能在旧数据清理未完成时继续混写，不影响原文读取

### Requirement: 问答先检索再生成
系统 SHALL 提供非流式 POST /api/ask，师生均可使用；仅用最新一句提问做本班 hybrid 检索，最多取前 4 个有效切片，有证据才调用独立对话网关。

#### Scenario: 有依据的简短回答
- **WHEN** 最新问题检索到本班相关切片
- **THEN** 模型仅接收该批材料标题、切片序号、MySQL chunk_text、最新问题和允许的有限历史，不接收向量分量或其他班级资料
- **AND** 返回简短 answer 与按证据次序排列的 citations，回答中的 [1]、[2] 与该次列表对应

#### Scenario: 无依据不调用模型
- **WHEN** 本班 hybrid 无候选，例如天气、比分或资料仅存在于 B 班而当前身份属于 A 班
- **THEN** HTTP 200，answer 为“资料中未找到相关内容”，hits=[]、citations=[]，对话网关调用次数为 0
- **AND** 不能以模型参数记忆回答，不能构造出处

#### Scenario: 有限历史与客户端角色注入
- **WHEN** 请求携带历史消息、system/developer/tool 消息或伪造 class_id
- **THEN** 检索 query 仍只包含最新问题，客户端班级及非 user/assistant 角色被丢弃；服务端独立构造 system 指令，历史不是引用证据

#### Scenario: 引用校验与生成故障
- **WHEN** 模型输出引用超过本次证据编号、无引用回答或网关故障
- **THEN** 不作为成功答案返回；引用校验失败按设计返回 502，服务故障/超时返回 503，不冒充无命中、不编造答案
- **AND** 验收逐句核对回答与原片段，不将编号有效视为语义必定正确

### Requirement: 检索诊断与课程验收证据
系统 SHALL 提供模式、两路分数/名次与 RRF 诊断，便于区分词匹配与语义匹配；页面不展示向量分量，凭证不返回浏览器。

#### Scenario: 模式对照
- **WHEN** 使用原词与同义改写分别请求三种模式
- **THEN** 可对照 keyword_score/rank、vector_score/rank、rrf_score，缺席一路为 null；query_vector 元数据表明是否生成及维度，keyword 为 false/0，不包含浮点数组

#### Scenario: 两库与失败证据
- **WHEN** 核对同一切片的 MySQL 与 Qdrant 记录，或故意使嵌入失败
- **THEN** 成功记录的三处 ID 相同，payload 无 chunk_text；失败切片在 MySQL 保留且为 failed，没有对应向量；截图/日志不包含网关密钥
