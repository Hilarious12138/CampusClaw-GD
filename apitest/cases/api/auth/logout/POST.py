from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.login('peer');token=r.json()['access_token'];r=c.request('POST','/api/auth/logout',auth=None,headers={'Authorization':'Bearer '+token},expected=204);c.validate('POST','/api/auth/logout',r)
    import base64,json
    jti=json.loads(base64.urlsafe_b64decode(token.split('.')[1]+'=='))['jti'];assert c.sql('SELECT jti FROM revoked_tokens WHERE jti=%s',(jti,))
    c.request('GET','/api/me',auth=None,headers={'Authorization':'Bearer '+token},expected=401)
if __name__=='__main__':standalone(run)
