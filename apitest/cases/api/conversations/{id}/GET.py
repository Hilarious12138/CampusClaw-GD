from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    cv=c.ensure_conversation();r=c.request('GET','/api/conversations/'+cv['id'],auth='student',expected=200);c.validate('GET','/api/conversations/{id}',r);stored=c.sql('SELECT id,content,status,citations FROM conversation_messages WHERE conversation_id=%s',(cv['id'],));byid={m['id']:m for m in stored}
    import json
    for m in r.json()['messages']:
        row=byid[m['id']];assert row['content']==m['content'] and row['status']==m['status'] and json.loads(row['citations'])==m['citations']
    for auth in ['peer','other','teacher']:c.not_found('GET','/api/conversations/'+cv['id'],auth=auth)
if __name__=='__main__':standalone(run)
