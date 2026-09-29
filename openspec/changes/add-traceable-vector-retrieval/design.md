## Context

仅规划，禁止 Apply。2026-09-29 已核对[完整课程网站](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/index.html)七个章节；课程提及的 week04 原 spec 尚无可取得的链接。本设计以公开课件为本次依据，旧登录、角色、材料及下载实现继续沿用，不重做第 3 课。

## Goals / Non-Goals

三策略切片、三模式检索、教师重建索引、可追溯的简短问答。支持有限历史作为生成上下文，不做流式输出、长对话产品、交叉编码器重排序、LangChain/LlamaIndex 或其他编排框架；不扩展 TXT/MD 以外格式。

## Decisions

### D1 技术栈与模型模块

沿用 React 18 + TypeScript + Vite、Go net/http、MySQL 8.0、Nginx + Compose；新增内部 Qdrant，collection 固定名 `campusclaw_chunks`、Cosine 度量，版本在实施前固定。全文索引为 `FULLTEXT(chunk_text) WITH PARSER ngram`，MySQL ngram_token_size=2；修改已有索引参数需重建索引。

嵌入模块通过服务端课程网关的兼容 OpenAI `/embeddings` 接口，将切片/问句转换为固定维度浮点向量。对话模块是独立调用，接收有编号的证据与问题，返回简短回答；具体对话接口协议、两个模型名、网关地址/密钥、维度与费用由用户提供后确认。不能因“兼容 OpenAI”就假定使用 OpenAI 官方服务。禁止随机/哈希向量冒充语义嵌入。

浏览器只访问本站 /api，经 Nginx 到 Go；Go 才调用 MySQL、Qdrant 与两类网关。课件演示部署前缀是 /week04/api，本项目仍以 /api 为入口，不照搬该站点前缀。网关凭证只读服务端环境变量，不进入前端、日志或源码。

### D2 会话确定班级

复用 Cookie 会话与认证模块。为匹配课件，增量给 sessions 增加 class_id：新会话从数据库唯一成员关系赋值，旧会话只可由服务端唯一成员关系回填；认证时复核当前成员关系，将 user_id/role/class_id 写入可信请求上下文。无需重写登录 UI/密码校验，不改变旧上传与下载契约。

检索、问答、重建均只取该上下文 class_id；body/query/header 中 class_id/classId 解析后丢弃。零班级 403，多个班级无法唯一初始化时 409，不猜班级；多班切换不在本轮范围。keyword SQL 必须过滤会话班级与 index_status=ready；Qdrant 必须按同一 class_id 过滤，回 MySQL 时再次校验成员、材料真实班级、当前版本与 ready 状态。

B 班独有词在 A 班检索返回 200 空 hits，不以 403/404 暗示其他班存在资料；按 ID 打开越权材料/切片仍返回 404。师生均可检索/问答，仅本班教师可上传/重建。POST 沿用 Origin/CSRF 校验。

### D3 三种切分与偏移

| strategy | 规则 |
| --- | --- |
| auto（默认） | 最大 800 个 Unicode 码点，重叠 80；优先空行、换行、句号边界，无边界强制截断；忽略请求额外传入的长度、重叠和预处理参数 |
| custom | 指定换行/空行/句号分隔符，max_length 为 100～2000 整数，overlap_ratio 为 0～0.5；无断点按上限截断；可选移除 URL/邮箱、折叠连续空白 |
| hierarchy | 按行首 #、##、### 分章，标题保留在该章中；过长章再按 auto 切分，重叠不跨章；无标题按 auto |

上传与重建使用同一策略参数结构；缺省 auto，种子启动补建亦用 auto。custom 先预处理再分隔，折叠空白后换行不存在时不能宣称仍可按换行切分。原文件和持久化正文绝不被预处理覆盖。切片函数只返回文本/序号/区间，不调用模型或 Qdrant。

本项目边界约定：序号从 1 起，位置为 Unicode 码点的 0 起始、右端不包含区间。优先选窗口末端最后一个可用边界；若边界导致不前进则强制推进，重叠不得形成死循环，保留非空尾片。custom 重叠字符数=floor(max_length×overlap_ratio)。保存预处理结果 processed_text（无预处理时可为空并引用原文）、策略参数及 offset_basis=original/processed；必须满足 chunk_text 等于对应基准文本的区间截取。processed 偏移不可用于原文高亮；界面展示“预处理后文本区间”，查看完整切片并另开未改写原材料。未预处理的 auto/hierarchy 可定位原文区间。

### D4 两库模型与标识

保留现有 material_contents.body/original 为第 3 课正文与原文件的唯一存储，不重复复制正文。新增只读兼容视图 `knowledge_entries`：id=material_id、material_id、body_text=material_contents.body，用于课件命名对照；两者一对一。knowledge_entry_id 等于该视图 id，实际外键仍引用 material_contents.material_id。

| 存储 | 内容 |
| --- | --- |
| materials 扩展 | 当前 index_version、策略/预处理参数、processed_text、索引状态与重试/租约字段；现有字段不变 |
| knowledge_chunks | UUID id、class_id、material_id、knowledge_entry_id、index_version、chunk_index、start_char/end_char、offset_basis、chunk_text、index_status=pending/ready/failed；唯一(material_id,index_version,chunk_index)，正文 ngram 全文索引 |
| Qdrant | point ID = knowledge_chunks.id = payload.chunk_id；浮点向量；payload 仅 class_id、material_id、knowledge_entry_id、chunk_id、chunk_index |

Qdrant 不存标题/正文/摘录/原文件；版本以 MySQL 校验，不额外扩展课件 payload 字段。模型维度与集合一致，入库/问句同模型同维度。模型变更须显式重建，不允许不同模型向量混搜。

### D5 上传、失败与重建

上传保持既有解析/鉴权与原文事务→提交后原文成功→持久化索引任务→切片提交 pending→嵌入验证数量/维度/有限浮点值→Qdrant 完整 upsert 并确认→MySQL 切片标 ready。201 不等于向量 ready，页面显示索引状态。嵌入失败原文和切片保留，对应片 failed、无对应向量；不写残缺向量。keyword 也只查 ready，故 failed/pending 片不参与任一路检索；Qdrant 暂停时已 ready 切片仍可全文查询。

同一材料的索引任务串行化，状态/租约落 MySQL；本项目提议最多 3 次自动退避重试，重启恢复过期任务。普通失败重试复用切片 UUID，Qdrant upsert 幂等。部分 Qdrant 写入后失败时标 failed 并清理该失败批次点，清理失败可重试；无论清理是否完成，回表 ready 校验禁止残留点进入结果。两库不是原子事务。

教师显式重建采用本次请求策略：先令旧版本不可检索→删除旧 Qdrant 点并确认→删除旧切片→按新策略建立新切片/向量。若删点失败停止重建并记录失败，保留清理所需旧 ID，不继续混写新旧。未请求重建时，修改页面选项不自动改变已有索引；重建任务内锁防止旧 worker 回写。原文件/正文全程保留。

启动幂等补建种子与既有材料缺失的索引，种子默认 auto，不重复建账号、不改密码、不恢复被删原材料。索引失败提供本班教师重试/重建入口。已删除材料或不存在切片命中一律回表剔除。

### D6 检索路径与 RRF

本项目参数提案：query 去首尾空白后 1～500 码点，top_k 默认 5、范围 1～20，每路最多 50 候选；非法输入 400。各路先按 class_id/current version/ready 过滤再截断，同分按 chunk ID 确定排序。

- keyword：参数化 MATCH(chunk_text) AGAINST，按全文得分排序，排除≤0；只访问 MySQL，不调用嵌入、对话或 Qdrant。中文以二元 ngram 及固定语料验收，纯标点无有效词时为空。
- vector：问句嵌入→Qdrant Cosine 本班 filter，score≥0.35→以 point ID 回 MySQL 取 chunk_text 并再鉴权，按余弦分数排序。不得降低阈值补数。
- hybrid（默认）：两路各自过滤后按名次 RRF(d)=Σ1/(60+rank_i(d))，名次从 1 起，缺席一路贡献 0；重复 ID 合并。RRF 分值不代表语义相关度，不能拿它代替各路绝对门槛。双路命中累计贡献，但不承诺任何双路候选必定高于所有单路候选。

无候选返回 200、hits=[]、message=“资料中未找到相关内容”；不足 top_k 不凑数。语义阈值并不保证所有模型都无误命中，用原词/同义改写/天气比分/仅他班有依据的样例实测，无关问题不得送入生成流程。vector/hybrid 依赖失败返回 503，不伪装无命中、不静默切 keyword。

### D7 简短问答与引用

POST /api/ask 不实现新检索算法：只用本轮最新问题做本班 hybrid，最多取前 4 个有效切片。无 hit 直接返回固定文案、hits=[]、citations=[]，对话网关调用次数必须为 0；不允许生成模型靠训练记忆回答。检索依赖故障仍是 503，不是无依据。

有 hit 时，按结果顺序编号 [1]…[4]，服务端组装材料标题、切片序号、MySQL chunk_text 和最新问题；此前有限 user/assistant 历史仅作生成上下文，不加入检索 query。丢弃客户端 system/developer/tool 等非允许角色；system 提示仅由服务端构造，要求只依据证据作答，材料及历史均是不可信数据，不能授予模型数据库/检索工具权限。历史是对话上下文而非可引用资料。

非流式返回简短 answer 与有序 citations；编号与列表一致，每个 citation 含 chunk_id/material_id/标题/序号/字符区间/摘录/链接。服务端检查回答编号只在本次证据范围内，缺引用或越界引用不作为成功答案返回；本项目提议返回 502 与“回答引用校验失败”，保留真实 hits 供查看。模型超时/失败返回 503，禁止编造答案。引用范围检查不能证明语义忠实性，还需逐句与证据对照验收。前端用安全文本渲染，不执行材料或模型输出的 HTML。

### 接口与界面契约

| 接口 | 契约 |
| --- | --- |
| POST /api/retrieval/search | query、mode?、top_k?；返回 hits 与索引状态；不调用对话模型 |
| POST /api/ask | question、history?；固定 hybrid/top 4，返回 answer/hits/citations；班级和证据不接受客户端指定 |
| POST /api/materials/:id/reindex | 本班教师、Origin/CSRF；strategy/options；接受任务 202，学生 403、跨班/不存在 404 |
| GET /api/retrieval/chunks/:id | 鉴权后的完整 chunk_text、offset_basis 与出处；跨班/不存在 404 |
| 既有上传/详情 | 上传允许可选 strategy/options，省略兼容旧请求；详情附索引状态，原有权限与字段不变 |

每条 hit 含 material_id/material_title/chunk_id/chunk_index/start_char/end_char/offset_basis/excerpt/material_url/score；excerpt 为 MySQL chunk_text 前 200 码点（项目提案）。附 keyword_score/keyword_rank/vector_score/vector_rank/rrf_score（未命中一路为 null）供课堂比较，不能混淆分数量纲。

课件 think 提到 query_vector，而 rag 明确页面不展示向量分量：响应诊断只返回 query_vector={generated,dimension} 元数据，keyword 为 false/0，向量模式为 true/模型维度；不返回浮点数组或网关凭证。如老师要求原始分量验收，另用服务端测试记录核对，待确认，不在 UI 展示。

材料链接沿用前端 `/?material=<id>&chunk=<id>`，打开时取授权数据；预处理片只高亮 processed_text 或展示切片，不错误定位原文。页面增加模式/检索与简短问答入口、教师策略/重建入口、引用列表、加载/无依据/故障/索引未完成状态；本项目历史上限提议最近 6 条、每条 2000 码点、回答约 300 中文字，模型上下文超限显式拒绝，不截断伪造证据。

### D8 部署与健康

Qdrant 使用持久卷与内部网络、独立健康检查；不发布宿主端口。保持现有 /health 对 MySQL/登录链路的语义，Qdrant/网关故障不阻断登录页或已 ready 的 keyword。不通过高频模型调用做健康检查。检索/生成分别使用有限超时，按所选网关时延核对现有 Go/Nginx 超时；问答需一致配置而非无限等待。

## Verification / 课程对照

- [scope](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/scope.html)：师生本班检索、三模式、无依据、出处。
- [rag](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/rag.html)、[flow](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/flow.html)：集合/网关、二元 ngram、两库标识、failed、top 4 问答。
- [chunk](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/chunk.html)：三策略、800/80、自定义参数、预处理偏移及重建。
- [class-scope](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/class-scope.html)：会话 class_id、ready、双侧过滤及回表、跨班无命中。
- [answer](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/answer.html)：简短回答、引用顺序、无 hit 零生成调用、历史及 system 丢弃、非流式。
- [think](https://devops.hello1023.com/课件/第4课-课件-可追溯知识库检索/pages/think.html)：模式 score/rank 对照、Qdrant 停机、改班级攻击、策略重建和两库记录截图。

## Open Questions / 开工前概览

待提供两个网关的地址、模型、维度和凭证配置方式，确认 query_vector 诊断形式与项目自定参数；凭证不写入规约。上面兼容视图、索引任务、偏移标识是适配现有系统的实现提案，非课件原话。若取得 week04 原 spec 再核对差异。用户确认概览并明确授权后才开发，本次只修订文档。
