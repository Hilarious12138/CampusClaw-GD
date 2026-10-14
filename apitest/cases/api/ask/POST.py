from pathlib import Path
import sys
for parent in Path(__file__).resolve().parents:
    if (parent/'common.py').exists():sys.path.insert(0,str(parent));break
from common import standalone
def run(c):
    c.ensure_material();r=c.request('POST','/api/ask',auth='student',json={'question':'解方程时如何保持等式成立？'},expected=200);c.validate('POST','/api/ask',r);assert r.json()['citations'];assert '[1]' in r.json()['answer']
if __name__=='__main__':standalone(run)
