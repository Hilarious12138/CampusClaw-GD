from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    s=c.ensure_skill();r=c.request('POST','/api/skills',json={'source':c.source('第二条解题引导')},expected=201);c.validate('POST','/api/skills',r);row=c.sql('SELECT source,class_id FROM skills WHERE id=%s',(r.json()['id'],))[0];assert row['source']==r.json()['source'] and row['class_id']==c.classes[0]
    c.request('POST','/api/skills',json={'source':c.source()},expected=400);c.request('POST','/api/skills',auth='student',json={'source':c.source('学生越权')},expected=403);c.request('POST','/api/skills',json={'source':'bad'},expected=400)
if __name__=='__main__':standalone(run)
