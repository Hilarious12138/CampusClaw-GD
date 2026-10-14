from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    a=c.ensure_agent();r=c.request('GET','/api/agents',auth='student',expected=200);c.validate('GET','/api/agents',r)
    assert any(x['id']==a['id'] for x in r.json()['agents'])
    assert c.request('GET','/api/agents',auth='other',expected=200).json()['agents']==[]
    row=c.sql('SELECT class_id,prompt,enabled FROM tutor_agents WHERE id=%s',(a['id'],))[0]
    assert row['class_id']==c.classes[0] and row['prompt']==a['prompt'] and row['enabled']==1
if __name__=='__main__':standalone(run)
