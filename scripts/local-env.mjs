import { randomBytes } from "node:crypto";
import { existsSync, writeFileSync } from "node:fs";
if (existsSync(".env")) {
  console.log(".env 已存在，未覆盖。");
  process.exit(0);
}
const secret = () => randomBytes(24).toString("base64url");
const port = process.env.WEB_PORT || "3000";
const values = {
  MYSQL_DATABASE: "campusclaw",
  MYSQL_USER: "campusclaw",
  MYSQL_PASSWORD: secret(),
  MYSQL_ROOT_PASSWORD: secret(),
  JWT_SECRET: secret(),
  SEED_TEACHER_A_PASSWORD: secret(),
  SEED_STUDENT_A_PASSWORD: secret(),
  SEED_TEACHER_B_PASSWORD: secret(),
  SEED_STUDENT_B_PASSWORD: secret(),
  WEB_PORT: port,
  APP_ORIGIN: `http://127.0.0.1:${port}`,
  DEV_ORIGIN: "http://127.0.0.1:5173",
};
writeFileSync(
  ".env",
  Object.entries(values)
    .map(([k, v]) => `${k}=${v}`)
    .join("\n") + "\n",
  { mode: 0o600, flag: "wx" },
);
console.log("已生成仅服务端使用的 .env，演示密码请在该文件查看。");
