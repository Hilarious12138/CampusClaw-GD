import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { execFileSync } from "node:child_process";
import assert from "node:assert/strict";
for (const line of readFileSync(".env", "utf8").split("\n")) {
  const i = line.indexOf("=");
  if (i > 0 && !line.startsWith("#"))
    process.env[line.slice(0, i)] = line.slice(i + 1);
}
const docker = "/Applications/Docker.app/Contents/Resources/bin/docker";
const compose = (...args) =>
  execFileSync(docker, ["compose", ...args], {
    stdio: ["pipe", "pipe", "pipe"],
    encoding: "utf8",
  });
const origin = process.env.APP_ORIGIN;
const call = async (path, token, body) => {
  const r = await fetch(origin + path, {
    method: body ? "POST" : "GET",
    headers: {
      Origin: origin,
      ...(token ? { Authorization: "Bearer " + token } : {}),
      "Content-Type": "application/json",
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
    signal: AbortSignal.timeout(90000),
  });
  return { status: r.status, body: await r.json() };
};
const login = async () => {
  const r = await call("/api/auth/login", "", {
    username: "student_a",
    password: process.env.SEED_STUDENT_A_PASSWORD,
  });
  assert.equal(r.status, 200);
  return r.body.access_token;
};
const sql = (q) =>
  compose(
    "exec",
    "-T",
    "db",
    "sh",
    "-c",
    'MYSQL_PWD="$MYSQL_PASSWORD" mysql -N -B -u"$MYSQL_USER" "$MYSQL_DATABASE" -e "$1"',
    "sh",
    q,
  ).trim();
const evidence = { date: new Date().toISOString(), checks: [] };
try {
  let token = await login();
  const before = sql(
    "SELECT GROUP_CONCAT(id ORDER BY id) FROM knowledge_chunks WHERE index_status='ready'",
  );
  assert.ok(before);
  const modes = {};
  for (const mode of ["keyword", "vector", "hybrid"]) {
    const r = await call("/api/retrieval/search", token, {
      query: "借来的书多久需要还回去？",
      mode,
    });
    assert.equal(r.status, 200);
    modes[mode] = r.body;
  }
  evidence.modes = modes;
  compose("stop", "qdrant");
  token = await login();
  assert.equal((await call("/health")).status, 200);
  const keyword = await call("/api/retrieval/search", token, {
    query: "图书借阅",
    mode: "keyword",
  });
  assert.equal(keyword.status, 200);
  assert.ok(keyword.body.hits.length);
  for (const mode of ["vector", "hybrid"]) {
    const r = await call("/api/retrieval/search", token, {
      query: "图书借阅",
      mode,
    });
    assert.equal(r.status, 503);
  }
  evidence.checks.push(
    "Qdrant 停机：登录/health/keyword 正常；vector/hybrid 503",
  );
  compose("start", "qdrant");
  for (let i = 0; i < 20; i++) {
    const r = await call("/api/retrieval/search", token, {
      query: "图书借阅",
      mode: "vector",
    });
    if (r.status === 200) break;
    if (i === 19) throw Error("vector recovery");
    await new Promise((r) => setTimeout(r, 1000));
  }
  compose("restart", "api");
  for (let i = 0; i < 30; i++) {
    try {
      if ((await call("/health")).status === 200) break;
    } catch {}
    if (i === 29) throw Error("api recovery");
    await new Promise((r) => setTimeout(r, 1000));
  }
  const after = sql(
    "SELECT GROUP_CONCAT(id ORDER BY id) FROM knowledge_chunks WHERE index_status='ready'",
  );
  assert.equal(after, before);
  evidence.checks.push("API 重启：已有 ready 切片 UUID 保持不变；种子不重复");
  evidence.ngram_token_size = sql("SELECT @@ngram_token_size");
  assert.equal(evidence.ngram_token_size, "2");
  evidence.chunk_summary = sql(
    "SELECT index_status,COUNT(*) FROM knowledge_chunks GROUP BY index_status",
  );
  evidence.ports = compose("ps", "--format", "{{.Service}} {{.Ports}}").trim();
  // Only public IDs/diagnostics are written; never credentials or tokens.
  mkdirSync("evidence", { recursive: true });
  writeFileSync(
    "evidence/week04-verification.json",
    JSON.stringify(evidence, null, 2),
  );
  console.log(
    "PASS: real retrieval modes, Qdrant outage/recovery, restart idempotency, ngram=2; evidence saved.",
  );
} catch (e) {
  console.error(
    "FAIL:",
    e instanceof assert.AssertionError
      ? e.message
      : "运行验证失败；检查服务状态，不输出请求凭证。",
  );
  process.exitCode = 1;
} finally {
  try {
    compose("start", "qdrant", "api");
  } catch {
    console.error("服务恢复失败，请执行 docker compose up -d");
    process.exitCode = 1;
  }
}
