from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    cv=c.ensure_conversation();row=c.sql('SELECT user_id,class_id,agent_id FROM conversations WHERE id=%s',(cv['id'],))[0];student=c.request('GET','/api/me',auth='student',expected=200).json();assert row['user_id']==student['id'] and row['class_id']==c.classes[0] and row['agent_id']==c.ensure_agent()['id'];c.request('POST','/api/conversations',auth='other',json={'agent_id':c.ensure_agent()['id']},expected=404);c.request('POST','/api/conversations',auth='student',json={},expected=400)
if __name__=='__main__':standalone(run)
