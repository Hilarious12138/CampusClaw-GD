from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    a=c.ensure_agent();skill=c.ensure_skill()['id'];material=c.ensure_material()
    assert c.sql('SELECT skill_id FROM tutor_agent_skills WHERE agent_id=%s',(a['id'],))[0]['skill_id']==skill
    assert c.sql('SELECT material_id FROM tutor_agent_materials WHERE agent_id=%s',(a['id'],))[0]['material_id']==material
    c.request('POST','/api/agents',auth='student',json={'name':'越权','prompt':'','enabled':True,'skill_ids':[],'material_ids':[]},expected=403)
    foreign_skill=c.request('POST','/api/skills',auth='teacher_b',json={'source':c.source('B班专用解题引导')},expected=201).json()['id']
    from common import ident
    foreign_material=ident();teacher_b=c.request('GET','/api/me',auth='teacher_b',expected=200).json()['id']
    c.sql("INSERT INTO materials(id,class_id,uploader_id,filename,type,size_bytes) VALUES(%s,%s,%s,'B班私有.md','md',1)",(foreign_material,c.classes[1],teacher_b))
    # Existing foreign bindings and fabricated IDs must be indistinguishable.
    for wrong in [foreign_material, 'f'*32]:
        payload={'name':'错误绑定','prompt':'','enabled':True,'skill_ids':[],'material_ids':[wrong]}
        c.request('POST','/api/agents',json=payload,expected=404)
    c.request('POST','/api/agents',json={'name':'错误技能绑定','prompt':'','enabled':True,'skill_ids':[foreign_skill],'material_ids':[]},expected=404)
if __name__=='__main__':standalone(run)
