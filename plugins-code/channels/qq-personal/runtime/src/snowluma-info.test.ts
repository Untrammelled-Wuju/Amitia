import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import http from "node:http"
import test from "node:test"
import { changeWebuiPassword, evaluatePassword, extractLatestCredentials, readSnowLumaInfo, resolveDockerBin } from "./snowluma-info.js"

interface FakeDockerInput {
  webui: Record<string, unknown>
  vnc: string
  logs: string
}

function createFakeDocker(input: FakeDockerInput): { dir: string; bin: string } {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "snowluma-info-"))
  const webuiPath = path.join(dir, "webui.json")
  const vncPath = path.join(dir, "vnc-password")
  const logsPath = path.join(dir, "container.log")
  fs.writeFileSync(webuiPath, JSON.stringify(input.webui))
  fs.writeFileSync(vncPath, `${input.vnc}\n`)
  fs.writeFileSync(logsPath, input.logs)
  const bin = path.join(dir, "docker")
  fs.writeFileSync(
    bin,
    [
      "#!/bin/sh",
      'if [ "$1" = "exec" ]; then',
      `  case "$4" in`,
      `    /app/data/config/webui.json) cat "${webuiPath}" ;;`,
      `    /app/data/config/vnc-password) cat "${vncPath}" ;;`,
      "  esac",
      'elif [ "$1" = "logs" ]; then',
      `  cat "${logsPath}"`,
      "fi",
      "",
    ].join("\n"),
    { mode: 0o755 },
  )
  return { dir, bin }
}

test("reads the latest temporary credentials while the password is unchanged", async () => {
  const fake = createFakeDocker({
    webui: { mustChangePassword: true },
    vnc: "vnc-secret",
    logs: [
      "08:00:00 INFO [WebUI] initial credentials: user=admin password=first-temp",
      "09:00:00 INFO [WebUI] initial credentials: user=admin password=latest-temp",
    ].join("\n"),
  })
  const info = await readSnowLumaInfo({
    containerName: "snowluma",
    webuiPort: 5099,
    novncPort: 6081,
    dockerBin: fake.bin,
  })
  assert.equal(info.webuiUser, "admin")
  assert.equal(info.webuiPassword, "latest-temp")
  assert.equal(info.webuiPasswordState, "temporary")
  assert.equal(info.novncPassword, "vnc-secret")
  assert.equal(info.webuiUrl, "http://127.0.0.1:5099/")
  assert.equal(info.novncUrl, "http://127.0.0.1:6081/")
  assert.equal(info.error, "")
})

test("hides the password once the user has changed it", async () => {
  const fake = createFakeDocker({
    webui: { mustChangePassword: false },
    vnc: "vnc-secret",
    logs: "08:00:00 INFO [WebUI] initial credentials: user=admin password=stale-temp",
  })
  const info = await readSnowLumaInfo({
    containerName: "snowluma",
    webuiPort: 6000,
    novncPort: 6001,
    dockerBin: fake.bin,
  })
  assert.equal(info.webuiPasswordState, "changed")
  assert.equal(info.webuiPassword, "")
  assert.equal(info.novncPassword, "vnc-secret")
  assert.equal(info.webuiUrl, "http://127.0.0.1:6000/")
})

test("reports a readable error when docker is unavailable", async () => {
  const info = await readSnowLumaInfo({
    containerName: "snowluma",
    webuiPort: 5099,
    novncPort: 6081,
    dockerBin: path.join(os.tmpdir(), "definitely-missing-docker-binary"),
  })
  assert.equal(info.webuiPasswordState, "unavailable")
  assert.match(info.error, /docker/)
  assert.equal(info.webuiUrl, "http://127.0.0.1:5099/")
})

test("extracts only the last credential line", () => {
  const latest = extractLatestCredentials(
    [
      "initial credentials: user=admin password=one",
      "noise",
      "initial credentials: user=root password=two",
    ].join("\n"),
  )
  assert.deepEqual(latest, { user: "root", password: "two" })
  assert.equal(extractLatestCredentials("no credentials here"), null)
})

test("prefers an explicitly configured docker binary", () => {
  assert.equal(resolveDockerBin("/custom/docker"), "/custom/docker")
  assert.ok(resolveDockerBin("").length > 0)
})

test("prefers the recorded password over the temporary one", async () => {
  const fake = createFakeDocker({
    webui: { mustChangePassword: true },
    vnc: "vnc-secret",
    logs: "08:00:00 INFO [WebUI] initial credentials: user=admin password=temp-value",
  })
  const info = await readSnowLumaInfo({
    containerName: "snowluma",
    webuiPort: 5099,
    novncPort: 6081,
    dockerBin: fake.bin,
  }, "RecordedPass!1")
  assert.equal(info.webuiPassword, "RecordedPass!1")
  assert.equal(info.webuiPasswordState, "stored")
  assert.equal(info.error, "")
})

test("evaluates the snowluma password rules", () => {
  assert.equal(evaluatePassword("Passw0rd!ab").valid, true)
  const weak = evaluatePassword("weak")
  assert.equal(weak.valid, false)
  assert.deepEqual(weak.rules.filter((rule) => !rule.ok).map((rule) => rule.id), ["length", "upper", "special"])
  assert.equal(evaluatePassword("Has Space1!").rules.find((rule) => rule.id === "no-space")?.ok, false)
})

test("logs in first and then changes the webui password", async () => {
  const seen: Array<{ url: string; token: string; body: Record<string, unknown> }> = []
  const server = http.createServer((request, response) => {
    let body = ""
    request.on("data", (chunk) => { body += String(chunk) })
    request.on("end", () => {
      seen.push({
        url: String(request.url),
        token: String(request.headers["x-snowluma-token"] || ""),
        body: JSON.parse(body || "{}"),
      })
      response.setHeader("Content-Type", "application/json")
      if (request.url === "/api/login") {
        response.end(JSON.stringify({ success: true, token: "session-token-1", mustChangePassword: false }))
        return
      }
      if (request.url === "/api/auth/change-password") {
        response.end(JSON.stringify({ success: true, requireRelogin: true }))
        return
      }
      response.statusCode = 404
      response.end("{}")
    })
  })
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve))
  const address = server.address()
  const port = typeof address === "object" && address ? address.port : 0
  try {
    const result = await changeWebuiPassword({ webuiPort: port }, "OldPass!12", "NewPass!34")
    assert.equal(result.ok, true)
    assert.equal(result.message, "密码已更新")
    assert.equal(seen.length, 2)
    assert.deepEqual(seen[0].body, { username: "admin", password: "OldPass!12" })
    assert.equal(seen[1].url, "/api/auth/change-password")
    assert.equal(seen[1].token, "session-token-1")
    assert.deepEqual(seen[1].body, { oldPassword: "OldPass!12", newPassword: "NewPass!34" })
  } finally {
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

test("reports a wrong current password when the login is rejected", async () => {
  const server = http.createServer((request, response) => {
    request.resume()
    response.statusCode = 401
    response.setHeader("Content-Type", "application/json")
    response.end(JSON.stringify({ status: "failed", message: "Unauthorized" }))
  })
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve))
  const address = server.address()
  const port = typeof address === "object" && address ? address.port : 0
  try {
    const result = await changeWebuiPassword({ webuiPort: port }, "wrong", "NewPass!34")
    assert.equal(result.ok, false)
    assert.equal(result.message, "当前密码不正确")
  } finally {
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

test("surfaces the snowluma message when the change step rejects", async () => {
  const server = http.createServer((request, response) => {
    request.resume()
    response.setHeader("Content-Type", "application/json")
    if (request.url === "/api/login") {
      response.end(JSON.stringify({ success: true, token: "session-token-2" }))
      return
    }
    response.statusCode = 400
    response.end(JSON.stringify({ success: false, message: "新密码不得与旧密码相同" }))
  })
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve))
  const address = server.address()
  const port = typeof address === "object" && address ? address.port : 0
  try {
    const result = await changeWebuiPassword({ webuiPort: port }, "OldPass!12", "OldPass!12")
    assert.equal(result.ok, false)
    assert.equal(result.message, "新密码不得与旧密码相同")
  } finally {
    await new Promise<void>((resolve) => server.close(() => resolve()))
  }
})

test("reports a connection error when the webui is unreachable", async () => {
  const result = await changeWebuiPassword({ webuiPort: 1, timeoutMs: 800 }, "a", "b")
  assert.equal(result.ok, false)
  assert.match(result.message, /无法连接|超时/)
})
