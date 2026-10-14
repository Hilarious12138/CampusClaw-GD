from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    s=c.ensure_skill();r=c.request('GET','/api/skills',auth='student',expected=200);c.validate('GET','/api/skills',r);assert any(x['id']==s['id'] for x in r.json()['skills']);assert c.request('GET','/api/skills',auth='other',expected=200).json()['skills']==[]
if __name__=='__main__':standalone(run)
