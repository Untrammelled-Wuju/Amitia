import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"
import { inspectDockerService, startDockerService } from "./docker-service.js"

function fakeDocker(): { bin: string; marker: string } {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "qq-docker-service-"))
  const marker = path.join(root, "running")
  const bin = path.join(root, "docker")
  fs.writeFileSync(
    bin,
    [
      "#!/bin/sh",
      'if [ "$1" = "info" ]; then',
      `  if [ -f "${marker}" ]; then echo "27.0.1"; exit 0; fi`,
      '  echo "Cannot connect to the Docker daemon" >&2',
      "  exit 1",
      "fi",
      'if [ "$1" = "desktop" ] && [ "$2" = "start" ]; then',
      `  touch "${marker}"`,
      '  echo "Docker Desktop is starting"',
      "  exit 0",
      "fi",
      "exit 1",
      "",
    ].join("\n"),
    { mode: 0o755 },
  )
  return { bin, marker }
}

function fakeDockerWithContainer(): { bin: string; containerMarker: string } {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "qq-docker-container-"))
  const containerMarker = path.join(root, "container-running")
  const bin = path.join(root, "docker")
  fs.writeFileSync(
    bin,
    [
      "#!/bin/sh",
      'if [ "$1" = "info" ]; then echo "27.0.1"; exit 0; fi',
      'if [ "$1" = "inspect" ]; then',
      `  if [ -f "${containerMarker}" ]; then echo '{"Running":true,"Status":"running"}'; else echo '{"Running":false,"Status":"exited"}'; fi`,
      "  exit 0",
      "fi",
      'if [ "$1" = "start" ] && [ "$2" = "snowluma" ]; then',
      `  touch "${containerMarker}"`,
      "  exit 0",
      "fi",
      "exit 1",
      "",
    ].join("\n"),
    { mode: 0o755 },
  )
  return { bin, containerMarker }
}

test("detects an unavailable docker daemon", async () => {
  const fake = fakeDocker()
  const status = await inspectDockerService(fake.bin)
  assert.equal(status.available, false)
  assert.equal(status.error, "Docker 未运行")
})

test("starts Docker Desktop and waits until the daemon is ready", async () => {
  const fake = fakeDocker()
  const result = await startDockerService({
    dockerBin: fake.bin,
    startTimeoutMs: 2000,
    readyTimeoutMs: 2000,
    pollIntervalMs: 20,
  })
  assert.equal(result.started, true)
  assert.equal(result.alreadyRunning, false)
  assert.equal(result.version, "27.0.1")
  assert.equal(result.message, "Docker 已启动")
})

test("does not start an already running daemon", async () => {
  const fake = fakeDocker()
  fs.writeFileSync(fake.marker, "")
  const result = await startDockerService({ dockerBin: fake.bin })
  assert.equal(result.started, false)
  assert.equal(result.alreadyRunning, true)
  assert.equal(result.message, "Docker 已在运行")
})

test("starts a stopped SnowLuma container", async () => {
  const fake = fakeDockerWithContainer()
  const result = await startDockerService({
    dockerBin: fake.bin,
    containerName: "snowluma",
    readyTimeoutMs: 2000,
    pollIntervalMs: 20,
  })
  assert.equal(result.alreadyRunning, true)
  assert.equal(result.started, false)
  assert.equal(result.containerStarted, true)
  assert.equal(result.containerRunning, true)
  assert.equal(result.message, "SnowLuma 容器已启动")
})

test("reports a missing SnowLuma container", async () => {
  const fake = fakeDockerWithContainer()
  const bin = path.join(path.dirname(fake.bin), "missing-container-docker")
  fs.writeFileSync(
    bin,
    [
      "#!/bin/sh",
      'if [ "$1" = "info" ]; then echo "27.0.1"; exit 0; fi',
      'if [ "$1" = "inspect" ]; then echo "Error: No such object: snowluma" >&2; exit 1; fi',
      "exit 1",
      "",
    ].join("\n"),
    { mode: 0o755 },
  )
  await assert.rejects(
    () => startDockerService({ dockerBin: bin, containerName: "snowluma" }),
    /未找到 SnowLuma 容器/,
  )
})

test("reports a missing docker installation", async () => {
  await assert.rejects(
    () => startDockerService({ dockerBin: path.join(os.tmpdir(), "missing-docker-for-test") }),
    /未找到 Docker Desktop/,
  )
})
