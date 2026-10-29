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
const PIPE_OP_LOGIN_STATE = 7
const PIPE_OP_LOGIN_IDENTITY_HINT = 17
const PIPE_FLAG_LOGGED_IN = 1 << 2

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

function loginStateFrame(uin) {
  const frame = Buffer.alloc(HEADER_SIZE)
  frame.writeUInt32LE(PIPE_MAGIC, 0)
  frame.writeUInt16LE(PIPE_VERSION, 4)
  frame.writeUInt16LE(PIPE_OP_LOGIN_STATE, 6)
  frame.writeUInt32LE(PIPE_FLAG_LOGGED_IN, 16)
  frame.writeBigUInt64LE(BigInt(uin), 32)
  return frame
}

function wrapControlSocket(socket) {
  const write = socket.write.bind(socket)
  let injected = false
  socket.write = (chunk, ...args) => {
    if (!injected && Buffer.isBuffer(chunk) && chunk.length >= HEADER_SIZE &&
        chunk.readUInt32LE(0) === PIPE_MAGIC && chunk.readUInt16LE(4) === PIPE_VERSION &&
        chunk.readUInt16LE(6) === PIPE_OP_LOGIN_IDENTITY_HINT) {
      const uin = chunk.readBigUInt64LE(32)
      if (uin > 0n) {
        injected = true
        setTimeout(() => socket.emit("data", loginStateFrame(uin)), 100)
      }
    }
    return write(chunk, ...args)
  }
}

const createConnection = net.createConnection.bind(net)
net.createConnection = (...args) => {
  const socket = createConnection(...args)
  const target = args[0] && typeof args[0] === "object" ? args[0].path : args[0]
  if (/mojo\.\d+\.control\.sock$/.test(String(target || ""))) wrapControlSocket(socket)
  return socket
}

if (process.platform === "linux") {
  fs.readFileSync = readFileSync
  fs.readFile = readFile
  fs.promises.readFile = promiseReadFile
}

module.exports = { deviceInodeKey, loginStateFrame, syntheticLocks, syntheticProcessEnvironment }
