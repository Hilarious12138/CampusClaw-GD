from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    a=c.ensure_agent();path='/api/agents/'+a['id'];r=c.request('GET',path,auth='student',expected=200);c.validate('GET','/api/agents/{id}',r)
    assert r.json()['id']==a['id'] and r.json()['skill_ids']==a['skill_ids'] and r.json()['material_ids']==a['material_ids']
    c.not_found('GET',path,auth='other')
if __name__=='__main__':standalone(run)
