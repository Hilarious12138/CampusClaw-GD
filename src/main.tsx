import React, { useEffect, useState, useRef } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";
import { Conversations } from "./Conversations";
import { readEvents } from "./sse";
import { Retrieval, Strategy, Reindex, defaults } from "./Retrieval";
type Class = { id: string; name: string };
type User = {
  displayName: string;
  role: string;
  classes: Class[];
};
type Material = {
  id: string;
  filename: string;
  type: string;
  sizeBytes: number;
  uploaderName: string;
  createdAt: string;
  body?: string;
  index_state: string;
  index_error: string;
};
function App() {
  const [view,setView]=useState("knowledge");
  const entryLink = useRef(new URLSearchParams(location.search));
  const [chunkOptions, setChunkOptions] = useState(defaults);
  const accessToken = useRef("");
  const generation = useRef(0);
  const pending = useRef(new Set<AbortController>());
  const [expiresAt, setExpiresAt] = useState(0);
  function clearAuth() {
    generation.current++;
    for (const c of pending.current) c.abort();
    pending.current.clear();
    accessToken.current = "";
    setExpiresAt(0);
    setUser(null);
    setCid("");
    setItems([]);
    setDetail(null);
    setUpload(false);
    history.replaceState(null, "", "/login");
  }

  const [user, setUser] = useState<User | null>(null),
    [cid, setCid] = useState(""),
    [items, setItems] = useState<Material[]>([]),
    [detail, setDetail] = useState<Material | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [ready, setReady] = useState(false),
    [upload, setUpload] = useState(false);
  async function api(path: string, options: RequestInit = {}, binary = false) {
    const epoch = generation.current;
    const controller = new AbortController();
    pending.current.add(controller);
    const headers = new Headers(options.headers);
    if (accessToken.current)
      headers.set("Authorization", `Bearer ${accessToken.current}`);
    const r = await fetch(path, {
      ...options,
      credentials: "omit",
      headers,
      signal: controller.signal,
      cache: "no-store",
    })
      .catch(() => {
        if (epoch !== generation.current) throw Error("请求已取消。");
        setItems([]);
        setDetail(null);
        throw Error("无法连接服务，请稍后刷新。");
      })
      .finally(() => pending.current.delete(controller));
    if (epoch !== generation.current) throw Error("请求已取消。");
    if (!r.ok) {
      if (r.status === 401) {
        clearAuth();
      }
      if (r.status >= 500) {
        setItems([]);
        setDetail(null);
      }
      const b = await r.json().catch(() => null);
      throw Object.assign(
        Error(b?.error?.message || "服务暂时不可用，请稍后刷新。"),
        { hits: b?.hits },
      );
    }
    const body =
      r.status === 204 ? null : binary ? await r.blob() : await r.json();
    if (epoch !== generation.current) throw Error("请求已取消。");
    return body;
  }
  async function streamAPI(path:string, body:unknown, controller:AbortController, onEvent:(name:string,data:any)=>void) {
    const epoch=generation.current;pending.current.add(controller);
    try {
      const r=await fetch(path,{method:"POST",credentials:"omit",cache:"no-store",headers:{"Content-Type":"application/json",Authorization:`Bearer ${accessToken.current}`},body:JSON.stringify(body),signal:controller.signal});
      if(epoch!==generation.current)throw Error("请求已取消。");
      if(!r.ok){if(r.status===401)clearAuth();const b=await r.json().catch(()=>null);throw Error(b?.error?.message||"流式服务暂时不可用。");}
      await readEvents(r,(name,data)=>{if(epoch===generation.current)onEvent(name,data)});
    } finally {pending.current.delete(controller)}
  }
  async function list(id = cid) {
    const data = await api(`/api/classes/${id}/materials`);
    setItems(data.materials);
  }
  useEffect(() => {
    history.replaceState(null, "", "/login");
    setReady(true);
  }, []);
  useEffect(() => {
    if (!expiresAt) return;
    const timer = setTimeout(
      () => {
        clearAuth();
        setError("登录已过期，请重新登录。");
      },
      Math.max(0, expiresAt - Date.now()),
    );
    return () => clearTimeout(timer);
  }, [expiresAt]);
  useEffect(() => {
    setItems([]);
    setDetail(null);
    if (cid) list(cid).catch((e) => setError(e.message));
  }, [cid]);
  async function login(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      const result = await api("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(Object.fromEntries(f)),
      });
      accessToken.current = result.access_token;
      setExpiresAt(Date.now() + result.expires_in * 1000);
      setUser(result.user);
      setCid(result.user.classes[0]?.id || "");
      history.replaceState(null, "", "/");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(
        `/api/classes/${cid}/materials?options=${encodeURIComponent(JSON.stringify(chunkOptions))}`,
        {
          method: "POST",
          body: new FormData(e.currentTarget),
        },
      );
      await list();
      setUpload(false);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  if (!ready) return <div className="loading">正在连接 CampusClaw…</div>;
  if (!user)
    return (
      <main className="login">
        <section className="intro">
          <a className="brand">
            ◈ CampusClaw<span>GD</span>
          </a>
          <div>
            <span className="eyebrow">CONNECTED CLASSROOM</span>
            <h1>
              让知识汇聚，
              <br />
              让学习发生。
            </h1>
            <p>
              属于每个班级的教学知识空间。
              <br />
              在这里，共享材料，积累每一份成长。
            </p>
          </div>
          <small>CAMPUSCLAW · 智能教学平台 / 迭代 05</small>
        </section>
        <section className="signin">
          <form onSubmit={login}>
            <span className="eyebrow">WELCOME BACK</span>
            <h2>登录教学空间</h2>
            <p className="muted">使用教师或学生账号继续</p>
            <label>
              账号
              <input
                name="username"
                autoComplete="username"
                required
                placeholder="输入你的账号"
              />
            </label>
            <label>
              密码
              <input
                name="password"
                type="password"
                autoComplete="current-password"
                required
                placeholder="输入密码"
              />
            </label>
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            <button className="primary" disabled={busy}>
              {busy ? "正在登录…" : "登录 →"}
            </button>
            <p className="hint">账号由管理员预置；刷新页面后需重新登录。</p>
          </form>
        </section>
      </main>
    );
  return (
    <div className="shell">
      <aside>
        <a className="brand">
          ◈ CampusClaw<span>GD</span>
        </a>
        <div className="nav-label">教学空间</div>
        <button className="nav-item" onClick={()=>setView("knowledge")}>▤　班级知识库</button>
        <button className="nav-item" onClick={()=>setView("assistant")}>◇　解题助手</button>
        <a className="swagger-link" href="/swagger/" target="_blank" rel="noreferrer">接口文档 ↗</a>
        <div className="aside-bottom">
          <div className="avatar">{user.displayName.slice(0, 1)}</div>
          <strong>{user.displayName}</strong>
          <small>{user.role === "teacher" ? "教师" : "学生"}账号</small>
          <button
            onClick={async () => {
              try {
                await api("/api/auth/logout", {
                  method: "POST",
                });
                clearAuth();
              } catch (e) {
                setError((e as Error).message);
              }
            }}
          >
            退出登录
          </button>
        </div>
      </aside>
      <main className="workspace">
        <header>
          <span>
            教学空间 / <b>{view === "assistant" ? "解题助手" : "班级知识库"}</b>
          </span>
          <span className="badge">迭代 05</span>
        </header>
        <section className="heading">
          <div>
            <span className="eyebrow">{view === "assistant" ? "GUIDED LEARNING" : "CLASS KNOWLEDGE"}</span>
            <h1>{view === "assistant" ? "解题助手" : "班级知识库"}</h1>
            <p className="muted">{view === "assistant" ? "选择本班智能体，结合材料一步步思考。" : "让每一份教学材料，都有一个共同的归处。"}</p>
          </div>
          {view !== "assistant" && user.role === "teacher" && (
            <button
              className="primary"
              onClick={() => {
                setError("");
                setUpload(true);
              }}
              disabled={!cid}
            >
              ＋ 上传材料
            </button>
          )}
        </section>
        <div className="class-banner">
          <div className="class-icon">▧</div>
          <div>
            <small>当前班级</small>
            <select
              aria-label="当前班级"
              value={cid}
              onChange={(e) => setCid(e.target.value)}
            >
              {user.classes.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </div>
          <span>班级专属 · 安全共享</span>
        </div>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        {view==="assistant"?<Conversations key={cid} api={api} stream={streamAPI} teacher={user.role==="teacher"} classID={cid} openMaterial={async(id)=>{try{setDetail(await api(`/api/classes/${cid}/materials/${id}`))}catch(e){setError((e as Error).message)}}}/>:<>
        <Retrieval
          key={cid}
          initialChunk={entryLink.current.get("chunk") || undefined}
          api={api}
          openMaterial={async (id) => {
            try {
              setDetail(await api(`/api/classes/${cid}/materials/${id}`));
            } catch (e) {
              setError((e as Error).message);
            }
          }}
        />
        <div className="list-head">
          <h3>
            全部材料 <small>{items.length}</small>
          </h3>
          <button
            onClick={() => {
              setError("");
              list().catch((e) => setError(e.message));
            }}
          >
            ↻ 刷新列表
          </button>
        </div>
        <div className="materials">
          {items.map((m) => (
            <button
              className="material"
              key={m.id}
              onClick={async () => {
                try {
                  setDetail(await api(`/api/classes/${cid}/materials/${m.id}`));
                } catch (e) {
                  setError((e as Error).message);
                }
              }}
            >
              <span className="file-icon">{m.type.toUpperCase()}</span>
              <strong>{m.filename}</strong>
              <small>
                索引：
                {{
                  ready: "已就绪",
                  pending: "待处理",
                  indexing: "处理中",
                  failed: "失败，可重建",
                }[m.index_state] || "待处理"}
              </small>
              <span className="muted">
                {m.uploaderName} · {(m.sizeBytes / 1024).toFixed(1)} KB
              </span>
              <footer>
                {new Date(m.createdAt).toLocaleDateString("zh-CN")}
                <span>查看材料 ↗</span>
              </footer>
            </button>
          ))}
        </div>
        {!items.length && !error && (
          <div className="empty">
            暂无可显示的材料
            {user.role === "teacher"
              ? "，上传第一份教学材料吧。"
              : "，请等待老师上传。"}
          </div>
        )}
        <p className="footnote">
          仅展示你所属班级的材料 ·{" "}
          {user.role === "teacher"
            ? "支持上传 UTF-8 TXT / Markdown"
            : "学生可查看和下载材料"}
        </p>
        </>}
      </main>
      {(upload || detail) && (
        <div className="overlay">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label={upload ? "上传教学材料" : "材料正文"}
          >
            <button
              className="close"
              aria-label="关闭"
              onClick={() => {
                setUpload(false);
                setDetail(null);
              }}
            >
              ×
            </button>
            {upload ? (
              <form onSubmit={submit}>
                <h2>上传教学材料</h2>
                <p className="muted">文件解析后存入当前班级知识库。</p>
                <label className="drop">
                  选择文件
                  <input name="file" type="file" accept=".txt,.md" required />
                </label>
                <p className="hint">
                  UTF-8 编码 · TXT / Markdown · 最大 10 MiB
                </p>
                <Strategy value={chunkOptions} onChange={setChunkOptions} />
                {error && (
                  <p role="alert" className="error">
                    {error}
                  </p>
                )}
                <button className="primary" disabled={busy}>
                  {busy ? "解析入库中…" : "上传并入库"}
                </button>
              </form>
            ) : (
              detail && (
                <>
                  <h2>{detail.filename}</h2>
                  <p className="muted">{detail.uploaderName}</p>
                  <p>
                    索引状态：{detail.index_state} {detail.index_error}
                  </p>
                  <pre>{detail.body}</pre>
                  {user.role === "teacher" && (
                    <Reindex
                      key={detail.id}
                      api={api}
                      id={detail.id}
                      onDone={() =>
                        void list().catch((e) => setError(e.message))
                      }
                    />
                  )}
                  <button
                    className="download"
                    onClick={async () => {
                      try {
                        const blob = await api(
                          `/api/classes/${cid}/materials/${detail.id}/file`,
                          {},
                          true,
                        );
                        const url = URL.createObjectURL(blob);
                        const link = document.createElement("a");
                        link.href = url;
                        link.download = detail.filename;
                        link.click();
                        setTimeout(() => URL.revokeObjectURL(url), 1000);
                      } catch (e) {
                        setError((e as Error).message);
                      }
                    }}
                  >
                    下载原文件 ↓
                  </button>
                </>
              )
            )}
          </section>
        </div>
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
