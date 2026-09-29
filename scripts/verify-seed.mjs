// Uses a disposable database on this project's MySQL; never modifies the application database.
import { execFileSync } from "node:child_process";
import assert from "node:assert/strict";
const docker =
  process.env.DOCKER_BIN ||
  "/Applications/Docker.app/Contents/Resources/bin/docker";
const dc = (...args) =>
  execFileSync(docker, ["compose", ...args], {
    encoding: "utf8",
    stdio: ["pipe", "pipe", "pipe"],
  });
const db = "campus_verify_" + Date.now();
const root = (q) =>
  execFileSync(
    docker,
    [
      "compose",
      "exec",
      "-T",
      "db",
      "sh",
      "-c",
      'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --default-character-set=utf8mb4 -N -uroot',
    ],
    { input: q, encoding: "utf8", stdio: ["pipe", "pipe", "pipe"] },
  ).trim();
const query = (q) => root(`USE ${db}; ${q}`);
try {
  root(
    `CREATE DATABASE ${db} CHARACTER SET utf8mb4; GRANT ALL ON ${db}.* TO 'campusclaw'@'%';`,
  );
  let failed = false;
  try {
    dc(
      "exec",
      "-T",
      "-e",
      `MYSQL_DATABASE=${db}`,
      "-e",
      "SEED_STUDENT_B_PASSWORD=",
      "api",
      "/app/campusclaw",
      "setup",
    );
  } catch {
    failed = true;
  }
  assert.ok(failed);
  assert.equal(query("SELECT COUNT(*) FROM users;"), "0");
  assert.equal(query("SELECT COUNT(*) FROM classes;"), "0");
  dc(
    "exec",
    "-T",
    "-e",
    `MYSQL_DATABASE=${db}`,
    "api",
    "/app/campusclaw",
    "setup",
  );
  assert.equal(query("SELECT COUNT(*) FROM users;"), "4");
  assert.equal(query("SELECT COUNT(*) FROM materials;"), "2");
  query(
    "DELETE FROM materials WHERE filename='B班-学习指南.md'; UPDATE classes SET name='已修改班级' WHERE code='A';",
  );
  dc(
    "exec",
    "-T",
    "-e",
    `MYSQL_DATABASE=${db}`,
    "api",
    "/app/campusclaw",
    "setup",
  );
  assert.equal(query("SELECT COUNT(*) FROM materials;"), "1");
  assert.equal(query("SELECT name FROM classes WHERE code='A';"), "已修改班级");
  for (const statement of [
    "INSERT INTO classes SELECT * FROM classes LIMIT 1",
    "INSERT INTO class_memberships VALUES('00000000000000000000000000000000','00000000000000000000000000000000')",
  ]) {
    let rejected = false;
    try {
      query(statement);
    } catch {
      rejected = true;
    }
    assert.ok(rejected);
  }
  console.log(
    "通过：缺失种子密码整体失败、初始数量、删除不恢复、修改不覆盖、唯一与外键约束",
  );
} finally {
  root(
    `DROP DATABASE IF EXISTS ${db}; REVOKE ALL ON ${db}.* FROM 'campusclaw'@'%';`,
  );
}
