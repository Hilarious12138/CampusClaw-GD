from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    mid=c.ensure_material();r=c.request('GET',f'/api/classes/{c.classes[0]}/materials/{mid}',auth='student',expected=200);c.validate('GET','/api/classes/{classId}/materials/{id}',r);assert r.json()['body']==c.sql('SELECT body FROM material_contents WHERE material_id=%s',(mid,))[0]['body'];c.request('GET',f'/api/classes/{c.classes[0]}/materials/{mid}',auth='other',expected=404)
if __name__=='__main__':standalone(run)
