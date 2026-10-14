from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    mid=c.ensure_material();assert c.sql('SELECT material_id FROM material_contents WHERE material_id=%s',(mid,));assert c.sql('SELECT state FROM material_indexes WHERE material_id=%s',(mid,))[0]['state']=='ready'
    c.request('POST',f'/api/classes/{c.classes[0]}/materials',auth='student',files={'file':('bad.md',b'text','text/plain')},expected=403)
if __name__=='__main__':standalone(run)
