#include <atomic>
#include <cerrno>
#include <cstring>
#include <cstdlib>
#include <string>
#include <thread>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/stat.h>

// Amitia Linux WeChat preload transport.
//
// This library deliberately does NOT call version-specific WeChat internals
// until a target build has been independently verified. It establishes the
// in-process IPC boundary and reports its capabilities as fail-closed. Future
// version adapters implement the same JSON-line protocol without changing the
// .amitiax service or Native Agent contract.
static std::atomic<bool> running{true};

static std::string socket_path() {
  return "/tmp/amitia-wechat-hook-" + std::to_string(getpid()) + ".sock";
}

static std::string json_escape(const std::string& in) {
  std::string out;
  out.reserve(in.size());
  for (char c : in) {
    if (c == '\\' || c == '"') out.push_back('\\');
    if (c == '\n') { out += "\\n"; continue; }
    if (c == '\r') { out += "\\r"; continue; }
    out.push_back(c);
  }
  return out;
}

static std::string client_version() {
  const char* raw = std::getenv("AMITIA_WECHAT_CLIENT_VERSION");
  return raw ? std::string(raw) : std::string();
}

static std::string response_for(const std::string& request) {
  if (request.find("driver.capabilities") != std::string::npos) {
    const auto version = json_escape(client_version());
    return "{\"ok\":true,\"kind\":\"amitia-linux-preload\",\"version\":\"transport-v1\",\"clientVersion\":\"" + version + "\",\"capabilities\":{\"attached\":true,\"qr\":false,\"loginStatus\":false,\"selfProfile\":false,\"receiveText\":false,\"sendText\":false},\"message\":\"preload attached; target WeChat build has no verified protocol adapter\"}\n";
  }
  return "{\"ok\":false,\"error\":\"unsupported_driver_op\"}\n";
}

static void server() {
  const auto p = socket_path();
  unlink(p.c_str());
  int fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) return;

  sockaddr_un a{};
  a.sun_family = AF_UNIX;
  std::strncpy(a.sun_path, p.c_str(), sizeof(a.sun_path) - 1);
  if (bind(fd, reinterpret_cast<sockaddr*>(&a), sizeof(a)) != 0 || listen(fd, 8) != 0) {
    close(fd);
    unlink(p.c_str());
    return;
  }
  chmod(p.c_str(), 0600);

  while (running.load()) {
    int c = accept(fd, nullptr, nullptr);
    if (c < 0) {
      if (errno == EINTR) continue;
      break;
    }
    char buf[4096]{};
    const auto n = read(c, buf, sizeof(buf) - 1);
    const std::string q(buf, n > 0 ? static_cast<size_t>(n) : 0);
    const auto out = response_for(q);
    (void)write(c, out.data(), out.size());
    close(c);
  }

  close(fd);
  unlink(p.c_str());
}

__attribute__((constructor)) static void init() {
  std::thread(server).detach();
}

__attribute__((destructor)) static void fini() {
  running.store(false);
  unlink(socket_path().c_str());
}
