from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    mid=c.ensure_material();old=c.sql('SELECT id FROM knowledge_chunks WHERE material_id=%s',(mid,));r=c.request('POST',f'/api/materials/{mid}/reindex',json={'strategy':'hierarchy'},expected=202);c.validate('POST','/api/materials/{id}/reindex',r);c.wait_ready(mid);assert c.sql('SELECT id FROM knowledge_chunks WHERE material_id=%s',(mid,))!=old;c.request('POST',f'/api/materials/{mid}/reindex',auth='student',json={'strategy':'auto'},expected=403)
if __name__=='__main__':standalone(run)
