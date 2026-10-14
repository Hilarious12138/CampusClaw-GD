from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.request('GET','/swagger/',auth=None,expected=200);assert 'SwaggerUIBundle' in r.text and 'persistAuthorization:false' in r.text
    assert 'swagger-ui-bundle.js' in r.text
if __name__=='__main__':standalone(run)
