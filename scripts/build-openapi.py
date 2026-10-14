"""Author the single checked-in API contract. No secrets or runtime dependencies."""
import json, re
from pathlib import Path
route_source=Path('cmd/api/main.go').read_text()
annotated=set()
for method,path,registered in re.findall(r'// @API (GET|POST|PUT|DELETE) (\S+)\s*\n\s*mux\.HandleFunc\("([A-Z]+ /[^\"]+)"',route_source):
    assert registered==method+' '+path,('route annotation drift',method,path,registered)
    assert (method,path) not in annotated,('duplicate route annotation',method,path)
    annotated.add((method,path))
S={}
def obj(properties, required=None):return {'type':'object','properties':properties,'required':list(properties) if required is None else required}
def string(**kw):return {'type':'string',**kw}
def arr(item):return {'type':'array','items':item}
def ref(name):return {'$ref':'#/components/schemas/'+name}
ID=string(pattern='^[0-9a-f]{32}$');date=string(format='date-time');integer={'type':'integer'};number={'type':'number','nullable':True}
S['Error']=obj({'error':obj({'message':string()})})
S['Class']=obj({'id':ID,'code':string(),'name':string()})
S['User']=obj({'id':ID,'username':string(),'displayName':string(),'role':string(enum=['teacher','student']),'classes':arr(ref('Class'))})
S['Login']=obj({'access_token':string(),'token_type':string(enum=['Bearer']),'expires_in':integer,'user':ref('User')})
S['Hit']=obj({'material_id':ID,'material_title':string(),'chunk_id':string(format='uuid'),'chunk_index':integer,'start_char':integer,'end_char':integer,'offset_basis':string(enum=['original','processed']),'excerpt':string(),'material_url':string(),'score':{'type':'number'},'keyword_score':number,'keyword_rank':{'type':'integer','nullable':True},'vector_score':number,'vector_rank':{'type':'integer','nullable':True},'rrf_score':number})
S['Search']=obj({'hits':arr(ref('Hit')),'message':string(),'mode':string(enum=['keyword','vector','hybrid']),'query_vector':obj({'generated':{'type':'boolean'},'dimension':integer}),'index_status':{'type':'object','additionalProperties':integer}})
S['Material']=obj({'id':ID,'filename':string(),'type':string(enum=['txt','md']),'sizeBytes':integer,'createdAt':date,'uploaderName':string(),'index_state':string(enum=['pending','indexing','ready','failed']),'index_error':string(),'body':string()},['id','filename','type','sizeBytes','createdAt','uploaderName','index_state','index_error'])
S['Skill']=obj({'id':ID,'name':string(),'trigger':string(),'subject':string(),'description':string(),'enabled':{'type':'boolean'},'source':string(),'tools':arr(string(enum=['knowledge_search']))})
S['Agent']=obj({'id':ID,'name':string(),'prompt':string(),'enabled':{'type':'boolean'},'skill_ids':arr(ID),'material_ids':arr(ID)})
S['Conversation']=obj({'id':ID,'agent_id':ID,'title':string(),'created_at':date,'updated_at':date})
S['Message']=obj({'id':ID,'turn_id':ID,'role':string(enum=['user','assistant']),'content':string(),'status':string(enum=['generating','completed','failed','cancelled']),'citations':arr(ref('Hit')),'skill_name':string(),'created_at':date})
S['Prompt']=obj({'body':string(minLength=1,maxLength=2000)})
S['ChunkOptions']=obj({'strategy':string(enum=['auto','custom','hierarchy'],default='auto'),'max_length':{'type':'integer','minimum':100,'maximum':2000},'overlap_ratio':{'type':'number','minimum':0,'maximum':.5},'delimiter':string(enum=['newline','blankline','period']),'remove_urls':{'type':'boolean'},'remove_emails':{'type':'boolean'},'collapse_whitespace':{'type':'boolean'}},[])
paths={}
def op(method,path,title,response,status=200,body=None,public=False,teacher=False,errors=None,kind='application/json',description=''):
 if not path.startswith('/swagger/'):
  assert (method,path) in annotated,('OpenAPI operation lacks Go handler annotation',method,path)
 codes=errors or ([400,401,403,404,409,503] if not public else [400,403,503])
 desc=description+('\n仅本班教师可写；角色与班级以数据库为准。' if teacher else '')
 if not public:desc+='\nAuthorization: Bearer <JWT>；JWT 有效期30分钟，仅内存保存。写入请求还需 Origin 与 APP_ORIGIN/DEV_ORIGIN 一致（同源浏览器自动发送）。'
 responses={str(status):{'description':'成功' if status!=204 else '无响应体'}}
 if response is not None:responses[str(status)]['content']={kind:{'schema':response}}
 for code in codes:responses[str(code)]={'description':{400:'参数、配置或技能源文本无效',401:'未登录、令牌过期/撤销/篡改',403:'角色或请求来源不允许',404:'未找到资源；越权与不存在同体',409:'班级不唯一或会话并发',413:'上传过大',415:'不支持的文件格式/编码',422:'无有效正文',502:'引用校验失败（Nginx 的既有非流式代理可能映射为503）',503:'数据库、检索或模型暂时不可用'}[code],'content':{'application/json':{'schema':ref('Error')}}}
 item={'summary':title,'operationId':method.lower()+'_'+path.strip('/').replace('/','_').replace('{','').replace('}',''),'description':desc,'security':[] if public else [{'bearerAuth':[]}],'responses':responses,'tags':['教师配置' if teacher else '会话' if 'conversations' in path else '知识库' if any(x in path for x in ['materials','retrieval','ask']) else '系统与认证']}
 params=[]
 for part in path.split('/'):
  if part.startswith('{'):params.append({'in':'path','name':part[1:-1],'required':True,'schema':ID if part[1:-1]!='id' or 'chunks' not in path else string(format='uuid')})
 if params:item['parameters']=params
 if body is not None:item['requestBody']={'required':True,'content':{'application/json':{'schema':body}}}
 paths.setdefault(path,{})[method.lower()]=item
 return item
op('GET','/health','健康检查（只依赖 MySQL 及基础种子）',obj({'status':string(enum=['ok','unavailable'])}),public=True,errors=[503])
op('POST','/api/auth/login','账号密码登录',ref('Login'),body=obj({'username':string(maxLength=100),'password':string(format='password',maxLength=1024)}),public=True,errors=[401,403,503])
op('POST','/api/auth/logout','撤销当前 JWT',None,204,errors=[401,403,503])
op('GET','/api/me','读取当前用户与成员班级',ref('User'),errors=[401,503])
op('GET','/api/classes/{classId}/materials','列出本班材料',obj({'class':ref('Class'),'materials':arr(ref('Material'))}),errors=[401,404,503])
upload=op('POST','/api/classes/{classId}/materials','教师上传单个 UTF-8 TXT/MD 文件',obj({'id':ID}),201,teacher=True,errors=[400,401,403,404,413,415,422,503],description='最大10MiB；201表示原文入库，索引异步完成。options 为 JSON 字符串切片配置。')
upload['parameters'].append({'in':'query','name':'options','schema':string(),'description':'ChunkOptions 的 JSON 编码；省略为 auto。例如 {"strategy":"custom","max_length":800,"overlap_ratio":0.1}。'})
upload['requestBody']={'required':True,'content':{'multipart/form-data':{'schema':obj({'file':string(format='binary')})}}}
op('GET','/api/classes/{classId}/materials/{id}','读取本班材料正文',ref('Material'),errors=[401,404,503])
op('GET','/api/classes/{classId}/materials/{id}/file','鉴权下载原文件',string(format='binary'),kind='application/octet-stream',errors=[401,404,503])
op('POST','/api/materials/{id}/reindex','教师显式重建材料索引',obj({'state':string(enum=['pending'])}),202,body=ref('ChunkOptions'),teacher=True)
op('POST','/api/retrieval/search','三模式本班检索',ref('Search'),body=obj({'query':string(minLength=1,maxLength=500),'mode':string(enum=['keyword','vector','hybrid'],default='hybrid'),'top_k':{'type':'integer','minimum':1,'maximum':20,'default':5}},['query']),errors=[400,401,403,409,503])
op('GET','/api/retrieval/chunks/{id}','读取可追溯完整切片',{'allOf':[ref('Hit'),obj({'chunk_text':string()})]},errors=[401,403,404,409,503])
op('POST','/api/ask','第四课单次非流式问答',obj({'answer':string(),'hits':arr(ref('Hit')),'citations':arr(ref('Hit')),'index_status':{'type':'object','additionalProperties':integer}}),body=obj({'question':string(minLength=1,maxLength=500),'history':arr(obj({'role':string(),'content':string(maxLength=2000)}))},['question']),errors=[400,401,403,409,502,503])
op('GET','/api/prompt','读取本班旧提示词配置',ref('Prompt'),errors=[401,403,409,503],description='兼容第四课配置。第五课会话按所选智能体的角色提示词生成。')
op('PUT','/api/prompt','教师保存本班旧提示词配置',ref('Prompt'),body=ref('Prompt'),teacher=True,errors=[400,401,403,409,503],description='兼容第四课配置。不会更改已有或新建智能体的角色提示词。')
op('GET','/api/skills','列出本班技能',obj({'skills':arr(ref('Skill'))}),errors=[401,403,409,503])
op('POST','/api/skills','教师创建技能源文本',ref('Skill'),201,body=obj({'source':string(maxLength=8000,example='名称：解题引导\n适用条件：解题,方程\n资料：本班知识库\n工具：knowledge_search\n启用：是\n\n先询问学生已知条件，再提示下一步，不给最终答案。')}),teacher=True)
op('GET','/api/skills/{id}','按编号读取本班技能',ref('Skill'),errors=[401,403,404,409,503],description='他班技能和不存在技能返回相同的404。')
op('PUT','/api/skills/{id}','教师更新技能（同内容重复提交不新增）',ref('Skill'),body=obj({'source':string(maxLength=8000)}),teacher=True)
op('DELETE','/api/skills/{id}','教师删除技能（保留历史会话）',None,204,teacher=True)
op('GET','/api/agents','列出本班智能体（学生只见启用项）',obj({'agents':arr(ref('Agent'))}),errors=[401,403,409,503])
agentBody=obj({'name':string(minLength=1,maxLength=64),'prompt':string(maxLength=2000),'enabled':{'type':'boolean'},'skill_ids':arr(ID),'material_ids':arr(ID)},['name','prompt','enabled','skill_ids','material_ids'])
op('POST','/api/agents','教师创建智能体并绑定本班技能和材料',ref('Agent'),201,body=agentBody,teacher=True,errors=[400,401,403,404,409,503])
op('GET','/api/agents/{id}','读取本班智能体',ref('Agent'),errors=[401,403,404,409,503],description='学生读取停用、外班或不存在的智能体均返回相同的404。')
op('PUT','/api/agents/{id}','教师更新智能体提示词和绑定关系',ref('Agent'),body=agentBody,teacher=True,errors=[400,401,403,404,409,503])
op('GET','/api/conversations','列出我在本班的会话',obj({'conversations':arr(ref('Conversation'))}),errors=[401,403,409,503])
op('POST','/api/conversations','用启用智能体新建个人会话',ref('Conversation'),201,body=obj({'agent_id':ID}),description='必须指定本班启用智能体；创建后不可改选。会话始终只允许所属用户读取。')
op('GET','/api/conversations/{id}','读取我的会话及历史状态',obj({'conversation':ref('Conversation'),'messages':arr(ref('Message'))}),errors=[401,403,404,409,503])
sse=op('POST','/api/conversations/{id}/messages','发送问题并真正流式生成',string(),body=obj({'content':string(minLength=2,maxLength=500)}),kind='text/event-stream',description='依次 meta、delta(多次)、done。每轮按绑定技能的适用条件路由最多一条，只在智能体绑定的材料中检索；前4片再过滤空正文和技能资料主题。无证据仍调用模型回答，但citations为空且不允许输出虚构编号。流前用JSON错误；流后error终止，HTTP仍200且无done。并发发送409。不自动重试。停止/断连后读详情确认failed/cancelled。')
sse['responses']['200']['content']['text/event-stream']['example']='event: meta\ndata: {"turn_id":"...","status":"ok","skill_name":null,"citations":[],"index_status":{}}\n\nevent: delta\ndata: {"text":"先看已知条件。"}\n\nevent: done\ndata: {"turn_id":"...","message_id":"...","status":"completed"}\n\n'
sse['x-sse-events']={'meta':'turn_id/status(ok)/agent_id/skill_name/skill(对象或null)/citations/index_status','delta':'text','done':'turn_id/message_id/status(completed)','error':'turn_id/code/message；流后HTTP仍200，终止且不发done'}
op('GET','/openapi.json','读取 OpenAPI',{'type':'object'},public=True,errors=[503])
for path in ['/swagger/','/swagger/index.html']:
    op('GET',path,'打开本机 Swagger UI',string(),public=True,errors=[503],kind='text/html',description='无需登录；静态资源随镜像提供。Authorize 后可试调受保护接口，禁用令牌持久化。')
documented={(m.upper(),p) for p,operations in paths.items() for m in operations if not p.startswith('/swagger/')}
assert documented==annotated,('Go handler / OpenAPI route drift',documented^annotated)
paths['/health']['get']['responses']['503']['content']['application/json']['schema']=obj({'status':string(enum=['unavailable'])})
api={'openapi':'3.0.3','info':{'title':'CampusClaw-GD API','version':'0.5.0','description':'Bearer JWT；班级/角色查数据库。保留第四课接口，新增教师智能体、技能路由和个人流式会话。Swagger Authorize 只输入 Token，不输入 Bearer 前缀。写请求 Origin 须匹配服务端来源。现有非流式 Nginx 会将502/504映射503；流式路由不拦截，流后错误见error事件。所有ID为本项目hex32或切片UUID，与课程原型数字ID不同。'},'servers':[{'url':'/','description':'同源 Nginx 入口'}],'security':[{'bearerAuth':[]}],'paths':paths,'components':{'securitySchemes':{'bearerAuth':{'type':'http','scheme':'bearer','bearerFormat':'JWT'}},'schemas':S}}
Path('cmd/api/openapi.json').write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
print(f'OpenAPI: {sum(len(x) for x in paths.values())} operations')
