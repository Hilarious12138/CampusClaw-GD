from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    s=c.ensure_skill();source=c.source('验收解题引导改名');r=c.request('PUT','/api/skills/'+s['id'],json={'source':source},expected=200);c.validate('PUT','/api/skills/{id}',r);c.skill=r.json();assert c.sql('SELECT source FROM skills WHERE id=%s',(s['id'],))[0]['source']==source
    c.not_found('PUT','/api/skills/'+s['id'],auth='teacher_b',json={'source':source});c.request('PUT','/api/skills/'+s['id'],auth='student',json={'source':source},expected=403)
    c.request('PUT','/api/skills/'+s['id'],json={'source':source},expected=200);assert len(c.sql('SELECT id FROM skills WHERE id=%s',(s['id'],)))==1
if __name__=='__main__':standalone(run)
