from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.request('GET','/openapi.json',auth=None,expected=200);body=r.json();assert body['openapi']=='3.0.3';assert body['components']['securitySchemes']['bearerAuth']['scheme']=='bearer'
    for path,item in body['paths'].items():
        for method,op in item.items():
            assert op['security']==([] if path in ['/health','/api/auth/login','/openapi.json','/swagger/','/swagger/index.html'] else [{'bearerAuth':[]}])
if __name__=='__main__':standalone(run)
