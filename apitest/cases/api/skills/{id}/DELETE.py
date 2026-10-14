from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    s=c.request('POST','/api/skills',json={'source':c.source('待删除技能')},expected=201).json();c.not_found('DELETE','/api/skills/'+s['id'],auth='teacher_b');r=c.request('DELETE','/api/skills/'+s['id'],expected=204);c.validate('DELETE','/api/skills/{id}',r);assert not c.sql('SELECT id FROM skills WHERE id=%s',(s['id'],));c.request('DELETE','/api/skills/'+s['id'],expected=404)
if __name__=='__main__':standalone(run)
