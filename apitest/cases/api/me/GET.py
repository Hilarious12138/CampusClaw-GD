from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.request('GET','/api/me',auth='student',expected=200);c.validate('GET','/api/me',r);body=r.json();assert c.sql('SELECT role FROM users WHERE id=%s',(body['id'],))[0]['role']==body['role']
    c.request('GET','/api/me',auth=None,expected=401);c.request('GET','/api/me?token=ignored',auth=None,headers={'Cookie':'campus_session=ignored'},expected=401)
if __name__=='__main__':standalone(run)
