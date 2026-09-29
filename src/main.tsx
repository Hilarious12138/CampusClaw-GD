import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";
type Class = { id: string; name: string };
type User = {
  displayName: string;
  role: string;
  csrfToken: string;
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
};
function App() {
  const [user, setUser] = useState<User | null>(null),
    [cid, setCid] = useState(""),
    [items, setItems] = useState<Material[]>([]),
    [detail, setDetail] = useState<Material | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [ready, setReady] = useState(false),
    [upload, setUpload] = useState(false);
  async function api(path: string, options: RequestInit = {}) {
    const r = await fetch(path, {
      ...options,
      credentials: "same-origin",
      cache: "no-store",
    }).catch(() => {
      setItems([]);
      setDetail(null);
      throw Error("无法连接服务，请稍后刷新。");
    });
    if (!r.ok) {
      if (r.status === 401) {
        setUser(null);
        setItems([]);
        setDetail(null);
        history.replaceState(null, "", "/login");
      }
      if (r.status >= 500) {
        setItems([]);
        setDetail(null);
      }
      const b = await r.json().catch(() => null);
      throw Error(b?.error?.message || "服务暂时不可用，请稍后刷新。");
    }
    return r.status === 204 ? null : r.json();
  }
  async function me() {
    const u: User = await api("/api/me");
    setUser(u);
    setCid(u.classes[0]?.id || "");
    history.replaceState(null, "", "/");
    return u;
  }
  async function list(id = cid) {
    const data = await api(`/api/classes/${id}/materials`);
    setItems(data.materials);
  }
  useEffect(() => {
    me()
      .catch((e) => {
        if (e.message !== "请先登录。") setError(e.message);
      })
      .finally(() => setReady(true));
  }, []);
  useEffect(() => {
    setItems([]);
    setDetail(null);
    if (cid) list(cid).catch((e) => setError(e.message));
  }, [cid]);
  useEffect(() => {
    if (!user) return;
    const timer = setInterval(() => {
      api("/api/me").catch((e) => setError(e.message));
    }, 10000);
    return () => clearInterval(timer);
  }, [user]);
  async function login(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      await api("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(Object.fromEntries(f)),
      });
      await me();
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
      await api(`/api/classes/${cid}/materials`, {
        method: "POST",
        headers: { "X-CSRF-Token": user!.csrfToken },
        body: new FormData(e.currentTarget),
      });
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
          <small>CAMPUSCLAW · 智能教学平台 / 迭代 01</small>
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
            <p className="hint">账号由管理员预置，暂不开放注册。</p>
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
        <div className="nav-item">▤　班级知识库</div>
        <div className="aside-bottom">
          <div className="avatar">{user.displayName.slice(0, 1)}</div>
          <strong>{user.displayName}</strong>
          <small>{user.role === "teacher" ? "教师" : "学生"}账号</small>
          <button
            onClick={async () => {
              try {
                await api("/api/auth/logout", {
                  method: "POST",
                  headers: { "X-CSRF-Token": user.csrfToken },
                });
                setUser(null);
                setItems([]);
                setDetail(null);
                history.replaceState(null, "", "/login");
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
            教学空间 / <b>班级知识库</b>
          </span>
          <span className="badge">迭代 01</span>
        </header>
        <section className="heading">
          <div>
            <span className="eyebrow">CLASS KNOWLEDGE</span>
            <h1>班级知识库</h1>
            <p className="muted">让每一份教学材料，都有一个共同的归处。</p>
          </div>
          {user.role === "teacher" && (
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
                  <pre>{detail.body}</pre>
                  <a
                    className="download"
                    href={`/api/classes/${cid}/materials/${detail.id}/file`}
                  >
                    下载原文件 ↓
                  </a>
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
