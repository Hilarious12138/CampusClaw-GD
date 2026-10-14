from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    c.ensure_material()
    for mode in ['keyword','vector','hybrid']:
        r=c.request('POST','/api/retrieval/search',auth='student',json={'query':'等式两边相等','mode':mode,'class_id':c.classes[1]},expected=200);c.validate('POST','/api/retrieval/search',r);assert r.json()['hits']
        for hit in r.json()['hits']:
            row=c.sql('SELECT chunk_text,class_id FROM knowledge_chunks WHERE id=%s',(hit['chunk_id'],))[0];assert row['class_id']==c.classes[0] and hit['excerpt']==row['chunk_text'][:200]
        out=c.request('POST','/api/retrieval/search',auth='other',json={'query':'等式两边相等','mode':mode,'class_id':c.classes[0]},headers={'X-Class-ID':c.classes[0]},expected=200).json();assert out['hits']==[]
    c.request('POST','/api/retrieval/search',json={'query':'bad','top_k':0},expected=400)
if __name__=='__main__':standalone(run)
