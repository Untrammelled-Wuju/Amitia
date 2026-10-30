const fs = require("node:fs")
const path = require("node:path")

const originalReadFile = fs.readFile.bind(fs)
const originalReadFileSync = fs.readFileSync.bind(fs)
const originalPromiseReadFile = fs.promises.readFile.bind(fs.promises)
const readDirectorySync = fs.readdirSync.bind(fs)
const readLinkSync = fs.readlinkSync.bind(fs)
const net = require("node:net")

const PIPE_MAGIC = 0x31504851
const PIPE_VERSION = 1
const HEADER_SIZE = 40

function attachFrameLog(socket) {
  let pending = Buffer.alloc(0)
  socket.on("data", chunk => {
    pending = pending.length ? Buffer.concat([pending, chunk]) : chunk
    while (pending.length >= HEADER_SIZE) {
      if (pending.readUInt32LE(0) !== PIPE_MAGIC || pending.readUInt16LE(4) !== PIPE_VERSION) {
        pending = Buffer.alloc(0)
        return
      }
      const cmdLength = pending.readUInt32LE(20)
      const msgLength = pending.readUInt32LE(24)
      const bodyLength = pending.readUInt32LE(28)
      const length = HEADER_SIZE + cmdLength + msgLength + bodyLength
      if (pending.length < length) return
      const op = pending.readUInt16LE(6)
      if (op === 5 || op === 7 || op === 22) {
        const message = pending.subarray(HEADER_SIZE + cmdLength, HEADER_SIZE + cmdLength + msgLength).toString("utf8").slice(0, 500)
        console.error(`[HookFrame] op=${op} status=${pending.readInt32LE(12)} flags=${pending.readUInt32LE(16)} detail=${message || "-"}`)
      } else if (op === 6 || op === 21) {
        const cmd = pending.subarray(HEADER_SIZE, HEADER_SIZE + cmdLength).toString("utf8")
        console.error(`[HookFrame] op=${op} status=${pending.readInt32LE(12)} cmd=${cmd || "-"} body=${bodyLength}`)
      }
      pending = pending.subarray(length)
    }
  })
}

function fileText(file) {
  if (typeof file === "string") return file
  if (Buffer.isBuffer(file)) return file.toString()
  if (file instanceof URL) return file.pathname
  return String(file)
}

function mainProcessIds() {
  const pids = []
  for (const entry of readDirectorySync("/proc", { withFileTypes: true })) {
    if (!entry.isDirectory() || !/^\d+$/.test(entry.name)) continue
    try {
      const cmdline = originalReadFileSync(`/proc/${entry.name}/cmdline`, "utf8")
      if (cmdline.includes("/opt/QQ/qq") && !cmdline.includes("--type=")) pids.push(Number(entry.name))
    } catch {}
  }
  return pids.sort((left, right) => left - right)
}

function heldDatabases(pids) {
  const files = new Map()
  for (const pid of pids) {
    let descriptors
    try {
      descriptors = readDirectorySync(`/proc/${pid}/fd`)
    } catch {
      continue
    }
    for (const descriptor of descriptors) {
      try {
        const file = readLinkSync(`/proc/${pid}/fd/${descriptor}`).replace(/ \(deleted\)$/, "")
        if (file.endsWith(".db") && file.includes("/.config/QQ/")) files.set(file, pid)
      } catch {}
    }
  }
  return files
}

function deviceInodeKey(device, inode) {
  const major = (device >> 8) & 0xfff
  const minor = (device & 0xff) | ((device >> 12) & 0xfff00)
  return `${major.toString(16).padStart(2, "0")}:${minor.toString(16).padStart(2, "0")}:${inode}`
}

function syntheticLocks() {
  const pids = mainProcessIds()
  if (!pids.length) return ""
  const lines = []
  for (const [file, pid] of heldDatabases(pids)) {
    try {
      const info = fs.statSync(file)
      const key = deviceInodeKey(info.dev, info.ino)
      lines.push(`${lines.length + 1}: POSIX  ADVISORY  WRITE ${pid} ${key} 0 EOF`)
    } catch {}
  }
  return lines.length ? `${lines.join("\n")}\n` : ""
}

function syntheticProcessEnvironment(file) {
  if (!/^\/proc\/\d+\/environ$/.test(fileText(file))) return null
  return `HOME=${process.env.HOME || ""}\0XDG_CONFIG_HOME=${path.join(process.env.HOME || "", ".config")}\0`
}

function readFileSync(file, options) {
  if (fileText(file) === "/proc/locks") {
    const text = syntheticLocks()
    if (options === "utf8" || options === "utf-8" || options?.encoding) return text
    return Buffer.from(text)
  }
  const environment = syntheticProcessEnvironment(file)
  if (environment !== null) {
    if (options === "utf8" || options === "utf-8" || options?.encoding) return environment
    return Buffer.from(environment)
  }
  return originalReadFileSync(file, options)
}

function readFile(file, options, callback) {
  if (typeof options === "function") {
    callback = options
    options = undefined
  }
  if (fileText(file) === "/proc/locks") {
    const value = readFileSync(file, options)
    process.nextTick(() => callback(null, value))
    return
  }
  if (syntheticProcessEnvironment(file) !== null) {
    const value = readFileSync(file, options)
    process.nextTick(() => callback(null, value))
    return
  }
  return originalReadFile(file, options, callback)
}

async function promiseReadFile(file, options) {
  if (fileText(file) === "/proc/locks") return readFileSync(file, options)
  if (syntheticProcessEnvironment(file) !== null) return readFileSync(file, options)
  return originalPromiseReadFile(file, options)
}

const createConnection = net.createConnection.bind(net)
net.createConnection = (...args) => {
  const socket = createConnection(...args)
  const target = args[0] && typeof args[0] === "object" ? args[0].path : args[0]
  if (/mojo\.\d+\.(control|recv)\.sock$/.test(String(target || ""))) attachFrameLog(socket)
  return socket
}

if (process.platform === "linux") {
  fs.readFileSync = readFileSync
  fs.readFile = readFile
  fs.promises.readFile = promiseReadFile
}

module.exports = { attachFrameLog, deviceInodeKey, syntheticLocks, syntheticProcessEnvironment }
