from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.request('GET','/health',auth=None,expected=200);assert r.json()['status']=='ok';c.validate('GET','/health',r)
    assert c.sql('SELECT COUNT(*) n FROM seed_versions WHERE version=1')[0]['n']==1
if __name__=='__main__':standalone(run)
