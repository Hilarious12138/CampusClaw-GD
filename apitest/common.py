"""Isolated test fixtures, schema/DB assertions and SSE decoding. Never log secrets."""
import json, os, re, sys, time, uuid
from pathlib import Path
import requests, pymysql, jsonschema
ROOT=Path(__file__).resolve().parents[1]
if (ROOT/'.env').exists():
    for line in (ROOT/'.env').read_text().splitlines():
        if '=' in line and not line.startswith('#'):
            key,value=line.split('=',1);os.environ.setdefault(key,value)
def ident():return uuid.uuid4().hex
def sanitize(value):
    text=str(value)
    for key,value in os.environ.items():
        if any(x in key for x in ['PASSWORD','SECRET','API_KEY']) and len(value)>5:text=text.replace(value,'[REDACTED]')
    text=re.sub(r'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+','[REDACTED JWT]',text)
    return text[:1600]
class Context:
    def __init__(self):
        self.base=os.getenv('APITEST_BASE_URL',os.getenv('APP_ORIGIN','http://127.0.0.1:3000')).rstrip('/')
        self.origin=os.environ['APP_ORIGIN'];self.http=requests.Session();self.http.trust_env=False
        self.db=pymysql.connect(host=os.getenv('APITEST_DB_HOST','db'),port=int(os.getenv('APITEST_DB_PORT','3306')),user=os.environ['MYSQL_USER'],password=os.environ['MYSQL_PASSWORD'],database=os.environ['MYSQL_DATABASE'],charset='utf8mb4',autocommit=True,cursorclass=pymysql.cursors.DictCursor)
        self.classes=[ident(),ident()];self.users=[];self.materials=[];self.tokens={};self.names={};self.skill=None;self.agent=None;self.conversation=None;self.chunk=None;self.document=None
        self.contract=self.request('GET','/openapi.json',auth=None).json()
        try:self.setup()
        except BaseException:self.close();raise
    def sql(self,q,args=()):
        with self.db.cursor() as c:c.execute(q,args);return c.fetchall()
    def request(self,method,path,auth='teacher',expected=None,**kw):
        headers={'Origin':self.origin,**kw.pop('headers',{})}
        if auth:headers.setdefault('Authorization','Bearer '+self.tokens[auth])
        r=self.http.request(method,self.base+path,headers=headers,timeout=(5,140),**kw)
        if expected is not None:assert r.status_code==expected,f'{method} {path.split("?")[0]}: expected {expected}, got {r.status_code}'
        return r
    def validate(self,method,template,r):
        operation=self.contract['paths'][template][method.lower()]
        assert str(r.status_code) in operation['responses'],'undocumented status'
        if r.status_code==204:assert not r.content;return
        media=operation['responses'][str(r.status_code)].get('content',{})
        if 'application/json' not in media:return
        def nullable(node):
            if isinstance(node,list):return [nullable(x) for x in node]
            if not isinstance(node,dict):return node
            result={k:nullable(v) for k,v in node.items() if k!='nullable'}
            if node.get('nullable') and 'type' in result:result['type']=[result['type'],'null']
            return result
        schema={'$ref':'#/response','response':nullable(media['application/json']['schema']),'components':nullable(self.contract['components'])}
        jsonschema.Draft7Validator(schema).validate(r.json())
    def login(self,role):
        return self.request('POST','/api/auth/login',auth=None,json={'username':self.names[role],'password':os.environ['SEED_'+self.base_accounts[role].upper()+'_PASSWORD']},expected=200)
    def setup(self):
        self.base_accounts={'teacher':'teacher_a','student':'student_a','other':'student_b','peer':'student_a','teacher_b':'teacher_b'}
        for i,cid in enumerate(self.classes):self.sql('INSERT INTO classes(id,code,name) VALUES(%s,%s,%s)',(cid,'test_'+cid[:12],'接口验收班'+str(i)))
        for role,base in self.base_accounts.items():
            row=self.sql('SELECT password_hash,role FROM users WHERE username=%s',(base,))[0];uid=ident();self.users.append(uid);self.names[role]='test_'+uid[:20]
            self.sql('INSERT INTO users VALUES(%s,%s,%s,%s,%s)',(uid,self.names[role],'接口验收'+role,row['password_hash'],row['role']))
            self.sql('INSERT INTO class_memberships VALUES(%s,%s)',(uid,self.classes[1] if role in ['other','teacher_b'] else self.classes[0]))
        for role in self.names:self.tokens[role]=self.login(role).json()['access_token']
        for cid in self.classes:self.sql('INSERT INTO class_prompts(class_id,body) VALUES(%s,%s)',(cid,'用简体中文，以问题和提示引导学生。'))
    def ensure_skill(self):
        if not self.skill:
            self.skill=self.request('POST','/api/skills',json={'source':self.source()},expected=201).json()
        return self.skill
    def source(self,name='验收解题引导',enabled=True):
        return f'名称：{name}\n适用条件：方程,解题\n资料：数学\n工具：knowledge_search\n启用：{"是" if enabled else "否"}\n\n先询问学生的已知条件，每次仅给下一步提示，不直接给最终数值或完整解答。引用本班证据。'
    def ensure_agent(self):
        if not self.agent:
            skill=self.ensure_skill()['id'];mid=self.ensure_material()
            self.agent=self.request('POST','/api/agents',json={'name':'验收数学助手','prompt':'用简体中文提供解题提示，并准确标注材料依据。','enabled':True,'skill_ids':[skill],'material_ids':[mid]},expected=201).json()
        return self.agent
    def ensure_conversation(self):
        if not self.conversation:self.conversation=self.request('POST','/api/conversations',auth='student',json={'agent_id':self.ensure_agent()['id']},expected=201).json()
        return self.conversation
    def ensure_material(self):
        if not self.document:
            body='# 一元一次方程引导\n\n解方程需要保持等式两边相等。可以在两边同时减去同一个数，或同时除以同一个非零数。先指出未知数与已知条件，再考虑先消去加减项还是乘除项。每一步都检查等式两边是否仍然相等。\n\n例如遇到两倍未知数再加一个常数的方程，先思考怎样消去常数项，不直接抄最终答案。'
            r=self.request('POST',f'/api/classes/{self.classes[0]}/materials',files={'file':('apitest-数学方程引导.md',body.encode(),'text/markdown')},expected=201);self.document=r.json()['id'];self.materials.append(self.document)
            self.wait_ready(self.document)
        return self.document
    def wait_ready(self,mid):
        for _ in range(90):
            row=self.sql('SELECT state FROM material_indexes WHERE material_id=%s',(mid,))[0]
            if row['state']=='ready':return
            assert row['state']!='failed','真实模型索引失败，不能跳过测试'
            time.sleep(2)
        raise AssertionError('index did not become ready')
    def ensure_chunk(self):
        self.ensure_material()
        r=self.request('POST','/api/retrieval/search',auth='student',json={'query':'等式两边相等','mode':'keyword'},expected=200)
        self.chunk=next(h for h in r.json()['hits'] if h['material_id']==self.document);return self.chunk
    def events(self,response):
        assert response.headers.get('Content-Type','').startswith('text/event-stream')
        event='message';data=[]
        response.encoding='utf-8'
        for line in response.iter_lines(chunk_size=1,decode_unicode=True):
            if line=='':
                if data:yield event,json.loads('\n'.join(data))
                event='message';data=[]
            elif line.startswith('event:'):event=line[6:].strip()
            elif line.startswith('data:'):data.append(line[5:].lstrip(' '))
        if data:yield event,json.loads('\n'.join(data))
    def not_found(self,method,path,**kw):
        actual=self.request(method,path,expected=404,**kw);missing=re.sub(r'/[0-9a-f]{32}(?=/|$)','/'+('f'*32),path)
        control=self.request(method,missing,expected=404,**kw)
        assert actual.json()==control.json(), '越权与不存在错误体不一致'
    def close(self):
        # Only this context's newly-created resources, never the demonstration classes.
        if hasattr(self,'db'):
            for cid in self.classes:
                mids=[r['id'] for r in self.sql('SELECT id FROM materials WHERE class_id=%s',(cid,))]
                if mids:
                    try:
                        q=os.getenv('APITEST_QDRANT_URL','http://qdrant:6333')
                        r=self.http.post(q+'/collections/campusclaw_chunks/points/delete?wait=true',json={'filter':{'must':[{'key':'class_id','match':{'value':cid}}]}},timeout=10)
                        assert r.ok
                    except Exception:print('测试向量清理失败，请检查 Qdrant。',file=sys.stderr)
                    self.sql('DELETE FROM materials WHERE class_id=%s',(cid,))
                self.sql('DELETE FROM classes WHERE id=%s',(cid,))
            for uid in self.users:self.sql('DELETE FROM users WHERE id=%s',(uid,))
            self.db.close();self.http.close()
def standalone(run):
    c=None
    try:c=Context();run(c);print('PASS')
    except Exception as e:print('FAIL: '+sanitize(e));sys.exit(1)
    finally:
        if c:c.close()
