import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import assert from "node:assert/strict";
import { chromium, request } from "@playwright/test";
const env = Object.fromEntries(
  readFileSync(".env", "utf8")
    .split("\n")
    .filter((x) => x && !x.startsWith("#"))
    .map((x) => {
      const i = x.indexOf("=");
      return [x.slice(0, i), x.slice(i + 1)];
    }),
);
const docker =
  process.env.DOCKER_BIN ||
  "/Applications/Docker.app/Contents/Resources/bin/docker";
const dc = (...args) =>
  execFileSync(docker, ["compose", ...args], {
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  });
const sql = (q, admin = false) =>
  execFileSync(
    docker,
    [
      "compose",
      "exec",
      "-T",
      "db",
      "sh",
      "-c",
      admin ? 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --default-character-set=utf8mb4 -N -uroot "$MYSQL_DATABASE"' : 'MYSQL_PWD="$MYSQL_PASSWORD" mysql --default-character-set=utf8mb4 -N -u"$MYSQL_USER" "$MYSQL_DATABASE"',
    ],
    { input: q, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"] },
  ).trim();
const wait = async (check, seconds = 60) => {
  for (let i = 0; i < seconds; i++) {
    if (await check()) return;
    await new Promise((r) => setTimeout(r, 1000));
  }
  throw Error("等待服务超时");
};
const origin = env.APP_ORIGIN;
const c = await request.newContext({
  baseURL: origin,
  extraHTTPHeaders: { Origin: origin },
});
assert.equal(
  (
    await c.post("/api/auth/login", {
      data: { username: "teacher_a", password: env.SEED_TEACHER_A_PASSWORD },
    })
  ).status(),
  200,
);
const me = await (await c.get("/api/me")).json(),
  cid = me.classes[0].id,
  url = `/api/classes/${cid}/materials`;
const counts = () =>
  sql(
    "SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM classes),(SELECT COUNT(*) FROM materials),(SELECT COUNT(*) FROM material_contents);",
  );
assert.equal(
  sql("SELECT COUNT(*) FROM users WHERE password_hash LIKE '$argon2id$%';"),
  "4",
);
const before = counts();
try {
  sql(
    "CREATE TRIGGER verify_reject BEFORE INSERT ON material_contents FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='verification rollback';", true,
  );
  const r = await c.post(url, {
    headers: { "X-CSRF-Token": me.csrfToken },
    multipart: {
      file: {
        name: "回滚验证.txt",
        mimeType: "text/plain",
        buffer: Buffer.from("must rollback"),
      },
    },
  });
  assert.equal(r.status(), 503);
  assert.equal(counts(), before);
} finally {
  sql("DROP TRIGGER IF EXISTS verify_reject;", true);
}
assert.equal(
  dc(
    "exec",
    "-T",
    "api",
    "sh",
    "-c",
    "find /tmp/campus-uploads -type f | wc -l",
  ).trim(),
  "0",
);
const originalName = sql(
  "SELECT display_name FROM users WHERE username='teacher_a';",
);
const hash = sql("SELECT password_hash FROM users WHERE username='teacher_a';");
try {
  sql("UPDATE users SET display_name='幂等验证' WHERE username='teacher_a';");
  dc("exec", "-T", "api", "/app/campusclaw", "setup");
  assert.equal(counts(), before);
  assert.equal(
    sql("SELECT display_name FROM users WHERE username='teacher_a';"),
    "幂等验证",
  );
  assert.equal(
    sql("SELECT password_hash FROM users WHERE username='teacher_a';"),
    hash,
  );
} finally {
  sql(
    `UPDATE users SET display_name='${originalName.replaceAll("'", "''")}' WHERE username='teacher_a';`,
  );
}
sql(
  "UPDATE sessions SET expires_at=UTC_TIMESTAMP()-INTERVAL 1 SECOND WHERE user_id=(SELECT id FROM users WHERE username='teacher_a');",
);
assert.equal((await c.get("/api/me")).status(), 401);
console.log(
  "通过：数据库密码哈希、事务失败回滚、临时文件清理、种子幂等、会话过期",
);
const browser = await chromium.launch({ channel: "chrome" });
const page = await browser.newPage();
try {
  await page.goto(origin);
  await page.getByLabel("账号", { exact: true }).fill("teacher_a");
  await page
    .getByLabel("密码", { exact: true })
    .fill(env.SEED_TEACHER_A_PASSWORD);
  await page.getByRole("button", { name: "登录 →" }).click();
  await page.getByRole("button", { name: /A班-学习指南/ }).waitFor();
  dc("stop", "api");
  assert.equal((await c.get("/")).status(), 503);
  assert.equal((await c.get("/api/me")).status(), 503);
  await page.getByRole("button", { name: "↻ 刷新列表" }).click();
  await page.getByRole("alert").waitFor();
  assert.equal(await page.locator(".material").count(), 0);
  console.log("通过：停 API 后入口 503，已打开页面清空材料");
} finally {
  dc("start", "api");
  await browser.close();
}
await wait(async () => (await c.get("/health")).status() === 200);
try {
  dc("stop", "db");
  await wait(async () => (await c.get("/health")).status() === 503);
  assert.deepEqual(await (await c.get("/health")).json(), {
    status: "unavailable",
  });
  await wait(
    async () =>
      JSON.parse(
        dc("ps", "--format", "json")
          .split("\n")
          .filter(Boolean)
          .find((x) => JSON.parse(x).Service === "api"),
      ).Health === "unhealthy",
  );
  console.log("通过：停库 /health 503，API unhealthy");
} finally {
  dc("start", "db");
}
await wait(async () => (await c.get("/health")).status() === 200);
dc("down");
dc("up", "-d");
await wait(async () => (await c.get("/health")).status() === 200);
assert.equal(counts(), before);
assert.equal(
  (
    await c.post("/api/auth/login", {
      data: { username: "teacher_a", password: env.SEED_TEACHER_A_PASSWORD },
    })
  ).status(),
  200,
);
assert.ok((await (await c.get(url)).json()).materials.length >= 2);
console.log("通过：数据库恢复及 Compose down/up 后账号、上传材料均保留");
await c.dispose();
