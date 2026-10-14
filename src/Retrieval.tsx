import { useState, useEffect } from "react";
export type API = (
  path: string,
  options?: RequestInit,
  binary?: boolean,
) => Promise<any>;
export type Options = {
  strategy: string;
  max_length: number;
  overlap_ratio: number;
  delimiter: string;
  remove_urls: boolean;
  remove_emails: boolean;
  collapse_whitespace: boolean;
};
export const defaults: Options = {
  strategy: "auto",
  max_length: 800,
  overlap_ratio: 0.1,
  delimiter: "newline",
  remove_urls: false,
  remove_emails: false,
  collapse_whitespace: false,
};
export function Strategy({
  value,
  onChange,
}: {
  value: Options;
  onChange: (o: Options) => void;
}) {
  const set = (key: string, v: unknown) => onChange({ ...value, [key]: v });
  return (
    <fieldset className="strategy">
      <legend>切片方式</legend>
      <select
        aria-label="切片方式"
        value={value.strategy}
        onChange={(e) => set("strategy", e.target.value)}
      >
        <option value="auto">自动 · 800 字 / 重叠 80 字</option>
        <option value="custom">自定义</option>
        <option value="hierarchy">Markdown 标题分章</option>
      </select>
      {value.strategy === "custom" && (
        <div className="custom-options">
          <label>
            长度
            <input
              type="number"
              min="100"
              max="2000"
              value={value.max_length}
              onChange={(e) => set("max_length", Number(e.target.value))}
            />
          </label>
          <label>
            重叠比例
            <input
              type="number"
              min="0"
              max="0.5"
              step="0.05"
              value={value.overlap_ratio}
              onChange={(e) => set("overlap_ratio", Number(e.target.value))}
            />
          </label>
          <label>
            分隔符
            <select
              value={value.delimiter}
              onChange={(e) => set("delimiter", e.target.value)}
            >
              <option value="newline">换行</option>
              <option value="blankline">空行</option>
              <option value="period">句号</option>
            </select>
          </label>
          {(
            ["remove_urls", "remove_emails", "collapse_whitespace"] as const
          ).map((key, i) => (
            <label className="check" key={key}>
              <input
                type="checkbox"
                checked={value[key]}
                onChange={(e) => set(key, e.target.checked)}
              />
              {["移除 URL", "移除邮箱", "折叠空白"][i]}
            </label>
          ))}
        </div>
      )}
      <small>保留原文件；预处理后的切片区间以处理后文本为准。</small>
    </fieldset>
  );
}
type Hit = {
  chunk_id: string;
  material_id: string;
  material_title: string;
  chunk_index: number;
  start_char: number;
  end_char: number;
  offset_basis: string;
  excerpt: string;
  score: number;
  keyword_score: number | null;
  keyword_rank: number | null;
  vector_score: number | null;
  vector_rank: number | null;
  rrf_score: number | null;
  chunk_text?: string;
};
export function Retrieval({
  api,
  openMaterial,
  initialChunk,
}: {
  api: API;
  initialChunk?: string;
  openMaterial: (id: string) => void;
}) {
  const [query, setQuery] = useState(""),
    [mode, setMode] = useState("hybrid"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [hits, setHits] = useState<Hit[]>([]),
    [answer, setAnswer] = useState(""),
    [message, setMessage] = useState(""),
    [chunk, setChunk] = useState<Hit | null>(null),
    [status, setStatus] = useState<Record<string, number>>({}),
    [meta, setMeta] = useState("");
  useEffect(() => {
    if (initialChunk)
      api("/api/retrieval/chunks/" + encodeURIComponent(initialChunk))
        .then(setChunk)
        .catch((e) => setError(e.message));
  }, [initialChunk]);
  async function run(ask: boolean) {
    if (!query.trim()) return;
    setBusy(true);
    setError("");
    setHits([]);
    setAnswer("");
    setMessage("");
    setChunk(null);
    setMeta("");
    setStatus({});
    try {
      const result = await api(ask ? "/api/ask" : "/api/retrieval/search", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(
          ask ? { question: query } : { query, mode, top_k: 5 },
        ),
      });
      setHits(result.hits);
      setAnswer(result.answer || "");
      setMessage(result.message || "");
      setStatus(result.index_status || {});
      if (result.query_vector)
        setMeta(
          `${result.mode} · 问句向量：${result.query_vector.generated ? result.query_vector.dimension + " 维" : "未生成"}`,
        );
    } catch (e) {
      setError((e as Error).message);
      setHits((e as Error & { hits?: Hit[] }).hits || []);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="retrieval">
      <div className="list-head">
        <h3>检索与问答</h3>
        <small>只使用本班资料</small>
      </div>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void run(false);
        }}
      >
        <label>
          想了解什么？
          <input
            maxLength={500}
            required
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="例如：本周有哪些学习目标？"
          />
        </label>
        <div className="retrieval-actions">
          <select
            aria-label="检索模式"
            value={mode}
            onChange={(e) => setMode(e.target.value)}
          >
            <option value="hybrid">混合检索 · 默认</option>
            <option value="keyword">关键词检索</option>
            <option value="vector">向量检索</option>
          </select>
          <button disabled={busy} className="primary">
            {busy ? "处理中…" : "检索片段"}
          </button>
          <button
            disabled={busy || !query.trim()}
            type="button"
            onClick={() => void run(true)}
          >
            基于资料回答
          </button>
        </div>
        <small>问答固定使用混合检索前 4 个片段；无依据时不生成答案。</small>
      </form>
      {(status.pending || 0) + (status.indexing || 0) + (status.failed || 0) >
        0 && (
        <p className="hint">
          索引处理中 {(status.pending || 0) + (status.indexing || 0)} 份，失败{" "}
          {status.failed || 0} 份。当前结果仅覆盖已就绪资料。
        </p>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {message && <p role="status">{message}</p>}
      {answer && (
        <div className="answer">
          <b>资料回答</b>
          <p>{answer}</p>
        </div>
      )}
      {meta && <small>{meta}</small>}
      <div className="hits">
        {hits.map((h, i) => (
          <article key={h.chunk_id}>
            <strong>
              [{i + 1}] {h.material_title}
            </strong>
            <small>
              切片 {h.chunk_index} · 字符 [{h.start_char}, {h.end_char}) ·{" "}
              {h.offset_basis === "processed" ? "预处理文本" : "原文"}
            </small>
            <p>{h.excerpt}</p>
            <details>
              <summary>检索分数与名次</summary>
              <p>
                关键词：{h.keyword_score?.toFixed(4) ?? "—"} / #
                {h.keyword_rank ?? "—"}；向量：
                {h.vector_score?.toFixed(4) ?? "—"} / #{h.vector_rank ?? "—"}
                ；RRF：{h.rrf_score?.toFixed(6) ?? "—"}
              </p>
            </details>
            <button
              onClick={async () => {
                try {
                  setChunk(await api("/api/retrieval/chunks/" + h.chunk_id));
                } catch (e) {
                  setError((e as Error).message);
                }
              }}
            >
              查看完整切片
            </button>{" "}
            <button onClick={() => openMaterial(h.material_id)}>
              打开原材料
            </button>
          </article>
        ))}
      </div>
      {chunk && (
        <div className="overlay">
          <section
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-label="切片正文"
          >
            <button
              className="close"
              aria-label="关闭切片"
              onClick={() => setChunk(null)}
            >
              ×
            </button>
            <h2>{chunk.material_title}</h2>
            <p>
              切片 {chunk.chunk_index} ·{" "}
              {chunk.offset_basis === "processed" ? "预处理文本" : "原文"}字符 [
              {chunk.start_char}, {chunk.end_char})
            </p>
            <pre>{chunk.chunk_text}</pre>
            <button
              onClick={() => {
                setChunk(null);
                openMaterial(chunk.material_id);
              }}
            >
              打开原材料
            </button>
          </section>
        </div>
      )}
    </section>
  );
}
export function Reindex({
  api,
  id,
  onDone,
}: {
  api: API;
  id: string;
  onDone: () => void;
}) {
  const [options, setOptions] = useState(defaults),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  return (
    <div className="reindex">
      <Strategy value={options} onChange={setOptions} />
      <button
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          setMessage("");
          try {
            await api(`/api/materials/${id}/reindex`, {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify(options),
            });
            setMessage("已提交重建；稍后刷新材料状态。");
            onDone();
          } catch (e) {
            setMessage((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      >
        按此策略重建索引
      </button>
      <p role="status">{message}</p>
    </div>
  );
}
