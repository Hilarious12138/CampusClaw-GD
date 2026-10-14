from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    mid=c.ensure_material();r=c.request('GET',f'/api/classes/{c.classes[0]}/materials/{mid}/file',auth='student',expected=200);assert r.content==c.sql('SELECT original FROM material_contents WHERE material_id=%s',(mid,))[0]['original'];assert 'attachment' in r.headers['Content-Disposition'];c.request('GET',f'/api/classes/{c.classes[0]}/materials/{mid}/file',auth=None,expected=401)
if __name__=='__main__':standalone(run)
