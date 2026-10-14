import html,importlib.util,json,sys,time
from pathlib import Path
from common import Context,sanitize
ROOT=Path(__file__).resolve().parent
ORDER=['/health','/openapi.json','/api/auth/login','/api/me','/api/prompt','/api/skills','/api/skills/{id}','/api/classes/{classId}/materials','/api/retrieval/search','/api/retrieval/chunks/{id}','/api/classes/{classId}/materials/{id}','/api/classes/{classId}/materials/{id}/file','/api/materials/{id}/reindex','/api/ask','/api/agents','/api/agents/{id}','/api/conversations','/api/conversations/{id}/messages','/api/conversations/{id}','/api/auth/logout']
def main():
    results=[];ctx=None
    try:
        ctx=Context()
        files=list((ROOT/'cases').rglob('*.py'))
        def key(p):
            url='/'+p.parent.relative_to(ROOT/'cases').as_posix();url=url+'/' if url=='/swagger' else url;return (ORDER.index(url) if url in ORDER else 100,{'GET':0,'PUT':1,'POST':2,'DELETE':3}.get(p.stem,4))
        files.sort(key=key)
        operations={(method.upper(),path) for path,item in ctx.contract['paths'].items() for method in item if method in ['get','post','put','delete','patch']}
        scripts={(p.stem,('/'+p.parent.relative_to(ROOT/'cases').as_posix()).replace('/swagger','/swagger/',1) if p.parent==ROOT/'cases'/'swagger' else '/'+p.parent.relative_to(ROOT/'cases').as_posix()) for p in files}
        assert operations==scripts,'OpenAPI 与按 URL 组织的测试脚本不一致'
        for i,path in enumerate(files):
            started=time.monotonic();label=path.stem+' /'+path.parent.relative_to(ROOT/'cases').as_posix()
            try:
                spec=importlib.util.spec_from_file_location('case_'+str(i),path);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);module.run(ctx)
                result=(label,True,time.monotonic()-started,'内容 / 权限 / Schema / 相关数据库断言通过')
            except Exception as e:result=(label,False,time.monotonic()-started,sanitize(e))
            results.append(result);print(('PASS ' if result[1] else 'FAIL ')+label,flush=True)
    except Exception as e:results.append(('初始化 / 契约覆盖',False,0,sanitize(e)));print('FAIL 初始化 / 契约覆盖')
    finally:
        if ctx:
            try:ctx.close()
            except Exception as e:results.append(('清理测试夹具',False,0,sanitize(e)))
        folder=ROOT/'reports';folder.mkdir(exist_ok=True)
        rows=''.join('<tr><td>'+html.escape(label)+'</td><td>'+('PASS' if ok else 'FAIL')+'</td><td>'+f'{duration:.2f}s'+'</td><td>'+html.escape(detail)+'</td></tr>' for label,ok,duration,detail in results)
        count=sum(r[1] for r in results)
        page=f'<!doctype html><meta charset="utf-8"><title>CampusClaw 接口验收</title><style>body{{font-family:system-ui;margin:40px;color:#203a35}}table{{border-collapse:collapse;width:100%}}td,th{{padding:12px;border:1px solid #dce8e3;text-align:left}}th{{background:#176b58;color:white}}</style><h1>CampusClaw-GD 接口验收</h1><p>{count} / {len(results)} 通过 · {time.strftime("%Y-%m-%d %H:%M:%S")}</p><p>隔离合成班级 / Bearer / 真实 MySQL / 真实模型；报告不含凭证。</p><table><tr><th>接口</th><th>结果</th><th>耗时</th><th>断言摘要</th></tr>{rows}</table>'
        (folder/'report.html').write_text(page,encoding='utf-8');(folder/'results.json').write_text(json.dumps(results,ensure_ascii=False,indent=2))
    return 0 if results and all(r[1] for r in results) else 1
if __name__=='__main__':sys.exit(main())
