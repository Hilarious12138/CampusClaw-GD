import {
  test,
  expect,
  request,
  type APIRequestContext,
} from "@playwright/test";
let a: APIRequestContext, b: APIRequestContext, s: APIRequestContext;
let ca: string, cb: string, csrf: string, uploaded: string;
test.beforeAll(async () => {
  async function login(username: string, key: string) {
    const c = await request.newContext({
      baseURL: process.env.APP_ORIGIN,
      extraHTTPHeaders: { Origin: process.env.APP_ORIGIN! },
    });
    expect(
      (
        await c.post("/api/auth/login", {
          data: { username, password: process.env[key] },
        })
      ).status(),
    ).toBe(200);
    return c;
  }
  a = await login("teacher_a", "SEED_TEACHER_A_PASSWORD");
  b = await login("teacher_b", "SEED_TEACHER_B_PASSWORD");
  s = await login("student_a", "SEED_STUDENT_A_PASSWORD");
  const me = await (await a.get("/api/me")).json();
  ca = me.classes[0].id;
  csrf = me.csrfToken;
  cb = (await (await b.get("/api/me")).json()).classes[0].id;
});
test.afterAll(async () => {
  await Promise.all([a?.dispose(), b?.dispose(), s?.dispose()]);
});
test("匿名访问与错误密码", async ({ request: r }) => {
  expect((await r.get("/api/me")).status()).toBe(401);
  expect((await r.get("/", { maxRedirects: 0 })).status()).toBe(302);
  expect(
    (
      await r.post("/api/auth/login", {
        headers: { Origin: process.env.APP_ORIGIN! },
        data: { username: "teacher_a", password: "incorrect" },
      })
    ).status(),
  ).toBe(401);
  expect((await r.get("/health")).status()).toBe(200);
});
test("角色、CSRF、服务端班级隔离", async () => {
  expect((await s.post(`/api/classes/${ca}/materials`)).status()).toBe(403);
  expect((await a.post(`/api/classes/${ca}/materials`)).status()).toBe(403);
  expect((await a.get(`/api/classes/${cb}/materials`)).status()).toBe(404);
  const bm = (await (await b.get(`/api/classes/${cb}/materials`)).json())
    .materials[0].id;
  for (const suffix of ["", "/file"])
    expect(
      (await a.get(`/api/classes/${cb}/materials/${bm}${suffix}`)).status(),
    ).toBe(404);
});
test("上传真实入库、正文与受保护下载", async () => {
  const body = "# 验证材料\n课堂学习记录";
  const r = await a.post(`/api/classes/${ca}/materials`, {
    headers: { "X-CSRF-Token": csrf },
    multipart: {
      file: {
        name: "迭代验收.md",
        mimeType: "text/markdown",
        buffer: Buffer.from(body),
      },
    },
  });
  expect(r.status()).toBe(201);
  uploaded = (await r.json()).id;
  expect(
    (await (await s.get(`/api/classes/${ca}/materials`)).json()).materials.some(
      (m: { id: string }) => m.id === uploaded,
    ),
  ).toBeTruthy();
  expect(
    (await (await s.get(`/api/classes/${ca}/materials/${uploaded}`)).json())
      .body,
  ).toBe(body);
  expect(
    await (await s.get(`/api/classes/${ca}/materials/${uploaded}/file`)).text(),
  ).toBe(body);
  expect(
    (await b.get(`/api/classes/${ca}/materials/${uploaded}/file`)).status(),
  ).toBe(404);
  expect((await a.get("/uploads/迭代验收.md")).status()).toBe(404);
});
test("拒绝无效文件，失败不新增记录", async () => {
  const url = `/api/classes/${ca}/materials`;
  const before = (await (await a.get(url)).json()).materials.length;
  for (const [name, buffer, status] of [
    ["empty.txt", Buffer.from("  "), 422],
    ["binary.txt", Buffer.from([255]), 415],
    ["a.pdf", Buffer.from("pdf"), 415],
    ["large.txt", Buffer.alloc(10 * 1024 * 1024 + 1, 65), 413],
  ] as const) {
    expect(
      (
        await a.post(url, {
          headers: { "X-CSRF-Token": csrf },
          multipart: {
            file: { name, mimeType: "application/octet-stream", buffer },
          },
        })
      ).status(),
    ).toBe(status);
  }
  expect((await (await a.get(url)).json()).materials.length).toBe(before);
});
test("浏览器登录、刷新与退出", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/login/);
  await page.getByLabel("账号", { exact: true }).fill("teacher_a");
  await page
    .getByLabel("密码", { exact: true })
    .fill(process.env.SEED_TEACHER_A_PASSWORD!);
  await page.getByRole("button", { name: "登录 →" }).click();
  await expect(page.getByRole("heading", { name: "班级知识库" })).toBeVisible();
  await expect(page.getByRole("button", { name: /上传材料/ })).toBeVisible();
  await page.reload();
  await expect(
    page.getByText("迭代验收.md", { exact: true }).first(),
  ).toBeVisible();
  await page.screenshot({ path: "test-results/workspace.png", fullPage: true });
  await page.getByRole("button", { name: "退出登录" }).click();
  await expect(
    page.getByRole("heading", { name: "登录教学空间" }),
  ).toBeVisible();
});
test("会话轮换、退出失效和伪造角色无效", async () => {
  const c = await request.newContext({
    baseURL: process.env.APP_ORIGIN,
    extraHTTPHeaders: { Origin: process.env.APP_ORIGIN! },
  });
  const credentials = {
    username: "student_a",
    password: process.env.SEED_STUDENT_A_PASSWORD,
    role: "teacher",
  };
  await c.post("/api/auth/login", { data: credentials });
  const old = (await c.storageState()).cookies[0];
  expect(old.httpOnly).toBeTruthy();
  expect(old.sameSite).toBe("Lax");
  expect(old.expires - Date.now() / 1000).toBeGreaterThan(28000);
  await c.post("/api/auth/login", { data: credentials });
  const fresh = (await c.storageState()).cookies[0];
  expect(fresh.value).not.toBe(old.value);
  const stale = await request.newContext({
    baseURL: process.env.APP_ORIGIN,
    extraHTTPHeaders: { Cookie: `${old.name}=${old.value}` },
  });
  expect((await stale.get("/api/me")).status()).toBe(401);
  const me = await (await c.get("/api/me")).json();
  expect(me.role).toBe("student");
  expect(JSON.stringify(me)).not.toContain("password");
  expect(
    (
      await c.post(`/api/classes/${ca}/materials`, {
        data: { role: "teacher" },
      })
    ).status(),
  ).toBe(403);
  expect(
    (
      await c.post("/api/auth/logout", {
        headers: { "X-CSRF-Token": me.csrfToken },
      })
    ).status(),
  ).toBe(204);
  expect((await c.get("/api/me")).status()).toBe(401);
  await c.dispose();
  await stale.dispose();
});

test("混用班级路径与材料 ID、教师跨班上传", async () => {
 const bm=(await (await b.get(`/api/classes/${cb}/materials`)).json()).materials[0].id;
 expect((await a.get(`/api/classes/${ca}/materials/${bm}`)).status()).toBe(404);
 expect((await a.post(`/api/classes/${cb}/materials`,{headers:{"X-CSRF-Token":csrf}})).status()).toBe(404);
});
