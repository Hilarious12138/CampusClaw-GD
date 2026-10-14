from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    c.ensure_material();path=f'/api/classes/{c.classes[0]}/materials';r=c.request('GET',path,auth='student',expected=200);c.validate('GET','/api/classes/{classId}/materials',r);assert [m['id'] for m in r.json()['materials']]==[row['id'] for row in c.sql('SELECT id FROM materials WHERE class_id=%s ORDER BY created_at DESC,id',(c.classes[0],))];c.request('GET',path,auth='other',expected=404)
if __name__=='__main__':standalone(run)
