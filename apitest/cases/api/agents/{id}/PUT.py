from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    a=c.ensure_agent();cv=c.ensure_conversation();path='/api/agents/'+a['id'];payload={k:a[k] for k in ['name','prompt','enabled','skill_ids','material_ids']}
    payload['prompt']='验收更新后的智能体提示词'
    r=c.request('PUT',path,json=payload,expected=200);c.validate('PUT','/api/agents/{id}',r)
    assert c.sql('SELECT prompt FROM tutor_agents WHERE id=%s',(a['id'],))[0]['prompt']==payload['prompt']
    payload['enabled']=False;c.request('PUT',path,json=payload,expected=200)
    c.not_found('GET',path,auth='student')
    c.request('GET','/api/conversations/'+cv['id'],auth='student',expected=200)
    c.request('POST','/api/conversations',auth='student',json={'agent_id':a['id']},expected=404)
    payload['enabled']=True;c.request('PUT',path,json=payload,expected=200);c.agent.update(payload)
if __name__=='__main__':standalone(run)
