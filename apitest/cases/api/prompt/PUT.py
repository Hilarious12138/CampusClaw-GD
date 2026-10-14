from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    body='先确认学生的思考，只给下一步提示。';r=c.request('PUT','/api/prompt',json={'body':body},expected=200);c.validate('PUT','/api/prompt',r);assert c.sql('SELECT body FROM class_prompts WHERE class_id=%s',(c.classes[0],))[0]['body']==body
    c.request('PUT','/api/prompt',auth='student',json={'body':'越权','role':'teacher','class_id':c.classes[1]},headers={'X-Role':'teacher'},expected=403)
    assert c.request('GET','/api/prompt',auth='other',expected=200).json()['body']!=body
if __name__=='__main__':standalone(run)
