from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    import json,re,time
    c.ensure_material();cv=c.ensure_conversation();path='/api/conversations/'+cv['id']+'/messages';start=time.monotonic();r=c.request('POST',path,auth='student',json={'content':'方程 2x + 3 = 11，我该先做什么？请只给下一步提示，不给最终答案。'},stream=True,expected=200);events=[];times=[]
    for name,value in c.events(r):events.append((name,value));times.append(time.monotonic()-start)
    r.close();assert events and events[0][0]=='meta','缺少 meta';assert events[-1][0]=='done' and not any(n=='error' for n,v in events),'流未成功结束：'+','.join(n+('('+v.get('code','')+')' if n=='error' else '') for n,v in events);assert events[0][1]['status']=='ok' and events[0][1]['skill_name']==c.ensure_skill()['name'],'技能路由错误';assert events[0][1]['citations'] and all(x['material_id']==c.document for x in events[0][1]['citations']),'未提供真实材料出处';chunks=[v['text'] for n,v in events if n=='delta'];assert len(chunks)>=2,'需要真实多次流增量';answer=''.join(chunks);assert all(1<=int(n)<=len(events[0][1]['citations']) for n in re.findall(r'\[(\d+)\]',answer)),'正文引用越界';assert not re.search(r'x\s*[=＝]\s*4',answer),'不得直接给最终答案';mid=events[-1][1]['message_id'];row=c.sql('SELECT content,status,citations FROM conversation_messages WHERE id=%s',(mid,))[0];assert row['content']==answer and row['status']=='completed','流与数据库正文状态不一致';assert json.loads(row['citations'])==events[0][1]['citations'],'流与数据库引用不一致';assert times[0]<times[-1],'未体现流式到达'
    for auth in ['peer','other']:c.not_found('POST',path,auth=auth,json={'content':'越权消息'})
    assert len(c.sql('SELECT id FROM conversation_messages WHERE conversation_id=%s',(cv['id'],)))==2
    empty=c.request('POST','/api/agents',json={'name':'无绑定资料验收助手','prompt':'用简体中文回答。','enabled':True,'skill_ids':[],'material_ids':[]},expected=201).json()
    separate=c.request('POST','/api/conversations',auth='student',json={'agent_id':empty['id']},expected=201).json()
    response=c.request('POST','/api/conversations/'+separate['id']+'/messages',auth='student',json={'content':'请说明怎样整理课堂笔记'},stream=True,expected=200)
    no_evidence=list(c.events(response));response.close()
    assert no_evidence[0][0]=='meta' and no_evidence[0][1]['status']=='ok' and no_evidence[0][1]['citations']==[] and no_evidence[0][1]['skill'] is None,'无证据元数据不正确'
    assert no_evidence[-1][0]=='done' and any(n=='delta' for n,v in no_evidence),'无证据仍须调用模型流式回答'
    plain=''.join(v['text'] for n,v in no_evidence if n=='delta')
    assert plain and not re.search(r'\[\d+\]',plain),'无证据时不得出现虚构编号'
    stored=c.sql('SELECT content,status,citations FROM conversation_messages WHERE id=%s',(no_evidence[-1][1]['message_id'],))[0]
    assert stored['content']==plain and stored['status']=='completed' and json.loads(stored['citations'])==[],'无证据回答必须落库且无引用'
if __name__=='__main__':standalone(run)
