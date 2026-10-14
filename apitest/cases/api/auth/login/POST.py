from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.login('student');c.validate('POST','/api/auth/login',r);data=r.json();assert data['token_type']=='Bearer' and data['expires_in']==1800
    row=c.sql('SELECT id,role,password_hash FROM users WHERE username=%s',(c.names['student'],))[0];assert data['user']['id']==row['id'] and row['role']=='student';assert row['password_hash'].startswith('$argon2id$');assert data['user']['classes'][0]['id']==c.classes[0]
    c.request('POST','/api/auth/login',auth=None,json={'username':c.names['student'],'password':'incorrect'},expected=401)
if __name__=='__main__':standalone(run)
