from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    h=c.ensure_chunk();r=c.request('GET','/api/retrieval/chunks/'+h['chunk_id'],auth='student',expected=200);c.validate('GET','/api/retrieval/chunks/{id}',r);assert r.json()['chunk_text']==c.sql('SELECT chunk_text FROM knowledge_chunks WHERE id=%s',(h['chunk_id'],))[0]['chunk_text'];c.request('GET','/api/retrieval/chunks/'+h['chunk_id'],auth='other',expected=404)
if __name__=='__main__':standalone(run)
