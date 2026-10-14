from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    s=c.ensure_skill();path='/api/skills/'+s['id']
    r=c.request('GET',path,auth='student',expected=200);c.validate('GET','/api/skills/{id}',r)
    assert r.json()['id']==s['id'] and r.json()['source']==c.sql('SELECT source FROM skills WHERE id=%s',(s['id'],))[0]['source']
    c.not_found('GET',path,auth='other')
if __name__=='__main__':standalone(run)
