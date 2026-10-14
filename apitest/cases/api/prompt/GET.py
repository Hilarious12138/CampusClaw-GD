from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    r=c.request('GET','/api/prompt',auth='student',expected=200);c.validate('GET','/api/prompt',r);assert r.json()['body']==c.sql('SELECT body FROM class_prompts WHERE class_id=%s',(c.classes[0],))[0]['body']
if __name__=='__main__':standalone(run)
