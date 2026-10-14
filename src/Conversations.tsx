import { useEffect, useRef, useState } from 'react';
import type { API } from './Retrieval';
export type Stream = (path: string, body: unknown, controller: AbortController, event: (name: string, data: any) => void) => Promise<void>;
type Skill = { id: string; name: string; enabled: boolean; source: string };
type Agent = { id: string; name: string; prompt: string; enabled: boolean; skill_ids: string[]; material_ids: string[] };
type Material = { id: string; filename: string };
type Conversation = { id: string; title: string; agent_id: string };
type ChatMessage = { id: string; role: string; content: string; status: string; skill_name: string; citations: any[] };
const skillTemplate = '名称：数学解题引导\n适用条件：解题,方程\n资料：数学\n工具：knowledge_search\n启用：是\n\n先询问学生已知条件和已尝试步骤，再给下一步提示，不直接给最终答案。引用本班资料说明方法。';
const emptyAgent = (): Agent => ({ id: '', name: '', prompt: '', enabled: true, skill_ids: [], material_ids: [] });
export function Conversations({ api, stream, teacher, classID, openMaterial }: { api: API; stream: Stream; teacher: boolean; classID: string; openMaterial: (id: string) => void }) {
  const [skills, setSkills] = useState<Skill[]>([]), [agents, setAgents] = useState<Agent[]>([]), [materials, setMaterials] = useState<Material[]>([]), [list, setList] = useState<Conversation[]>([]), [current, setCurrent] = useState(''), [agentID, setAgentID] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>([]), [question, setQuestion] = useState(''), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [agentDraft, setAgentDraft] = useState<Agent>(emptyAgent), [source, setSource] = useState(skillTemplate), [editID, setEditID] = useState(''), [saving, setSaving] = useState(false), [notice, setNotice] = useState(''), [chunk, setChunk] = useState<any>(null);
  const controller = useRef<AbortController | null>(null), epoch = useRef(0);
  async function refreshConfig() {
    const [agentResult, materialResult, skillResult] = await Promise.all([api('/api/agents'), api(`/api/classes/${classID}/materials`), teacher ? api('/api/skills') : Promise.resolve({ skills: [] })]);
    setAgents(agentResult.agents); setMaterials(materialResult.materials); setSkills(skillResult.skills);
    setAgentID(old => agentResult.agents.some((a: Agent) => a.id === old && a.enabled) ? old : agentResult.agents.find((a: Agent) => a.enabled)?.id || '');
  }
  async function refreshList() { setList((await api('/api/conversations')).conversations); }
  useEffect(() => { void Promise.all([refreshConfig(), refreshList()]).catch(e => setError(e.message)); return () => { epoch.current++; controller.current?.abort(); }; }, [classID]);
  async function open(id: string) { controller.current?.abort(); const generation = ++epoch.current; setCurrent(id); setMessages([]); setError(''); try { const detail = await api('/api/conversations/' + id); if (generation === epoch.current) setMessages(detail.messages); } catch (e) { if (generation === epoch.current) setError((e as Error).message); } }
  async function create() { setError(''); setSaving(true); try { const result = await api('/api/conversations', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ agent_id: agentID }) }); await refreshList(); await open(result.id); } catch (e) { setError((e as Error).message); } finally { setSaving(false); } }
  async function save(path: string, method: string, body?: unknown) { setSaving(true); setError(''); setNotice(''); try { await api(path, { method, headers: { 'Content-Type': 'application/json' }, ...(body ? { body: JSON.stringify(body) } : {}) }); setNotice('已保存到本班配置。'); await refreshConfig(); return true; } catch (e) { setError((e as Error).message); return false; } finally { setSaving(false); } }
  async function submit(e: React.FormEvent) {
    e.preventDefault(); if (!current || busy || question.trim().length < 2) return;
    const id = current, generation = ++epoch.current, abort = new AbortController(); controller.current = abort;
    setBusy(true); setError(''); const content = question.trim(); setQuestion('');
    const draft: ChatMessage = { id: 'draft', role: 'assistant', content: '', status: 'generating', skill_name: '', citations: [] };
    setMessages(old => [...old, { ...draft, id: 'user-draft', role: 'user', content, status: 'completed' }, draft]);
    try {
      await stream(`/api/conversations/${id}/messages`, { content }, abort, (name, value) => {
        if (generation !== epoch.current) return;
        setMessages(old => old.map(m => m.id !== 'draft' ? m : name === 'meta' ? { ...m, skill_name: value.skill_name, citations: value.citations } : name === 'delta' ? { ...m, content: m.content + value.text } : name === 'done' ? { ...m, status: 'completed' } : name === 'error' ? { ...m, status: 'failed' } : m));
      });
    } catch (e) { if (generation === epoch.current) setError(abort.signal.aborted ? '已停止生成；正在确认保存状态。' : (e as Error).message); }
    finally {
      if (generation === epoch.current) { setBusy(false); controller.current = null; try { setMessages((await api(`/api/conversations/${id}`)).messages); await refreshList(); } catch (e) { setMessages(old => old.map(m => m.id === 'draft' ? { ...m, status: 'failed' } : m)); setError((e as Error).message); } }
    }
  }
  return <section className="assistant-space">
    <div className="list-head"><h2>解题助手</h2><small>本班资料 · 分步引导 · 流式会话</small></div>
    {error && <p role="alert" className="error">{error}</p>}
    {teacher && <details className="teacher-config"><summary>教师：智能体与技能配置</summary>
      <h3>智能体</h3>
      <label>编辑智能体<select aria-label="编辑智能体" value={agentDraft.id} onChange={e => setAgentDraft(agents.find(a => a.id === e.target.value) || emptyAgent())}><option value="">新建智能体</option>{agents.map(a => <option key={a.id} value={a.id}>{a.name}{a.enabled ? '' : '（停用）'}</option>)}</select></label>
      <label>智能体名称<input aria-label="智能体名称" maxLength={64} value={agentDraft.name} onChange={e => setAgentDraft({ ...agentDraft, name: e.target.value })} /></label>
      <label>角色提示词<textarea aria-label="角色提示词" rows={4} maxLength={2000} value={agentDraft.prompt} onChange={e => setAgentDraft({ ...agentDraft, prompt: e.target.value })} /></label>
      <label><input type="checkbox" checked={agentDraft.enabled} onChange={e => setAgentDraft({ ...agentDraft, enabled: e.target.checked })} />启用智能体</label>
      <fieldset><legend>绑定技能</legend>{skills.map(s => <label key={s.id}><input type="checkbox" checked={agentDraft.skill_ids.includes(s.id)} onChange={e => setAgentDraft({ ...agentDraft, skill_ids: e.target.checked ? [...agentDraft.skill_ids, s.id] : agentDraft.skill_ids.filter(id => id !== s.id) })} />{s.name}{s.enabled ? '' : '（停用）'}</label>)}</fieldset>
      <fieldset><legend>绑定材料</legend>{materials.map(m => <label key={m.id}><input type="checkbox" checked={agentDraft.material_ids.includes(m.id)} onChange={e => setAgentDraft({ ...agentDraft, material_ids: e.target.checked ? [...agentDraft.material_ids, m.id] : agentDraft.material_ids.filter(id => id !== m.id) })} />{m.filename}</label>)}</fieldset>
      <button disabled={saving || !agentDraft.name.trim()} onClick={async () => { if (await save(agentDraft.id ? '/api/agents/' + agentDraft.id : '/api/agents', agentDraft.id ? 'PUT' : 'POST', { name: agentDraft.name, prompt: agentDraft.prompt, enabled: agentDraft.enabled, skill_ids: agentDraft.skill_ids, material_ids: agentDraft.material_ids })) setAgentDraft(emptyAgent()); }}>{agentDraft.id ? '保存智能体' : '创建智能体'}</button>
      <h3>技能</h3>
      <label>编辑技能<select aria-label="编辑技能" value={editID} onChange={e => { setEditID(e.target.value); setSource(skills.find(s => s.id === e.target.value)?.source || skillTemplate); }}><option value="">新建技能</option>{skills.map(s => <option key={s.id} value={s.id}>{s.name}{s.enabled ? '' : '（停用）'}</option>)}</select></label>
      <label>导入技能文本文件<input aria-label="导入技能文本文件" type="file" accept=".md,.txt,text/plain,text/markdown" onChange={async e => { const file=e.target.files?.[0]; if (!file) return; if (file.size > 32000) { setError('技能文件过大。'); return; } setSource(await file.text()); e.target.value=''; }} /></label>
      <label>技能源文本<textarea aria-label="技能源文本" rows={10} maxLength={8000} value={source} onChange={e => setSource(e.target.value)} /></label>
      <p className="hint">空行以上填写名称、适用条件、资料、工具、启用；空行以下写引导说明。工具仅支持 knowledge_search；启用为“是”或“否”。</p>
      <button disabled={saving} onClick={() => void save(editID ? '/api/skills/' + editID : '/api/skills', editID ? 'PUT' : 'POST', { source })}>{editID ? '保存技能' : '创建技能'}</button>
      {editID && <button disabled={saving} onClick={async () => { await save('/api/skills/' + editID, 'DELETE'); setEditID(''); setSource(skillTemplate); }}>删除选中技能</button>}
      {notice && <p role="status">{notice}</p>}
    </details>}
    <div className="chat-layout"><section className="chat-list"><h3>我的会话</h3>
      <select aria-label="会话智能体" value={agentID} onChange={e => setAgentID(e.target.value)}>{agents.filter(a => a.enabled).map(a => <option key={a.id} value={a.id}>{a.name}</option>)}</select>
      <button className="primary" disabled={busy || saving || !agentID} onClick={() => void create()}>＋ 新建会话</button>
      {list.map(c => <button key={c.id} disabled={busy} className={current === c.id ? 'selected-conversation' : ''} onClick={() => void open(c.id)}>{c.title}</button>)}
      {!agents.some(a => a.enabled) && <p className="hint">请等待教师启用智能体。</p>}
    </section><section className="chat-main">
      {!current ? <p className="empty">新建或选择一个会话，开始讨论解题思路。</p> : <>
        <div className="chat-messages" aria-live="polite">{messages.map(m => <article className={'chat-message ' + m.role} key={m.id}><strong>{m.role === 'user' ? '我' : m.skill_name || '解题助手'}</strong><small>{m.role === 'assistant' ? ({ generating: '生成中 · 未完成草稿', completed: '已完成', failed: '生成失败 · 草稿', cancelled: '已停止 · 草稿' }[m.status] || m.status) : ''}</small><p>{m.content || '正在检索本班材料…'}</p>
          {m.citations?.length > 0 && <details><summary>引用出处（{m.citations.length}）</summary>{m.citations.map((h, i) => <div className="chat-citation" key={h.chunk_id}><b>[{i + 1}] {h.material_title}</b><small>切片 {h.chunk_index} · [{h.start_char}, {h.end_char}) · {h.offset_basis === 'processed' ? '预处理文本' : '原文'}</small><p>{h.excerpt}</p><button onClick={async () => { try { setChunk(await api('/api/retrieval/chunks/' + h.chunk_id)); } catch (e) { setError((e as Error).message); } }}>查看切片</button> <button onClick={() => openMaterial(h.material_id)}>打开材料</button></div>)}</details>}
        </article>)}</div>
        <form className="chat-compose" onSubmit={submit}><label>你的问题<textarea aria-label="会话问题" rows={3} minLength={2} maxLength={500} value={question} onChange={e => setQuestion(e.target.value)} placeholder="说说题目的已知条件，以及你已经尝试的步骤。" disabled={busy} required /></label><button className="primary" disabled={busy || question.trim().length < 2}>发送问题</button>{busy && <button type="button" onClick={() => controller.current?.abort()}>停止生成</button>}<small>会先检索所选智能体绑定的本班资料；没有依据时仍可回答，但不会给出虚构引用。</small></form>
      </>}
    </section></div>
    {chunk && <div className="overlay"><section className="modal" role="dialog" aria-modal="true" aria-label="会话引用切片"><button className="close" onClick={() => setChunk(null)} aria-label="关闭切片">×</button><h2>{chunk.material_title}</h2><p>切片 {chunk.chunk_index} · [{chunk.start_char}, {chunk.end_char})</p><pre>{chunk.chunk_text}</pre></section></div>}
  </section>;
}
