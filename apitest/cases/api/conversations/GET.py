from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    cv=c.ensure_conversation();r=c.request('GET','/api/conversations',auth='student',expected=200);c.validate('GET','/api/conversations',r);assert any(x['id']==cv['id'] for x in r.json()['conversations']);assert c.request('GET','/api/conversations',auth='peer',expected=200).json()['conversations']==[]
if __name__=='__main__':standalone(run)
