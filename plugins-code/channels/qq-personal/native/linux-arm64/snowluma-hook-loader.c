#define _GNU_SOURCE

#include <dlfcn.h>
#include <elf.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <link.h>
#include <pthread.h>
#include <signal.h>
#include <spawn.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/socket.h>
#include <sys/uio.h>
#include <sys/prctl.h>
#include <sys/ptrace.h>
#include <sys/wait.h>
#include <unistd.h>

struct hook_start_request {
  char library[PATH_MAX];
};

static volatile sig_atomic_t hook_requested = 0;
static volatile int hook_launch_state = 0;
static char requested_hook_library[PATH_MAX];
static volatile uint64_t bridge_login_uin = 0;
static volatile int bridge_login_state_launching = 0;

ssize_t amitia_process_vm_readv(pid_t pid, const struct iovec *local_iov, unsigned long local_count, const struct iovec *remote_iov, unsigned long remote_count, unsigned long flags);
ssize_t amitia_process_vm_writev(pid_t pid, const struct iovec *local_iov, unsigned long local_count, const struct iovec *remote_iov, unsigned long remote_count, unsigned long flags);
ssize_t amitia_send(int socket, const void *buffer, size_t length, int flags);
ssize_t amitia_recv(int socket, void *buffer, size_t length, int flags);
static int redirect_vm_symbols(void *handle, const char *name, uintptr_t replacement);
static void verify_vm_fallback(void);
long amitia_ptrace(enum __ptrace_request request, ...);
static void append_log(const char *message, const char *detail);
typedef void (*receive_probe_fn)(void *, void *, int);
static receive_probe_fn receive_probe_original = NULL;
static volatile int receive_probe_installed = 0;
static volatile int receive_probe_count = 0;

static void append_hex_log(const char *label, const void *address, size_t length) {
  if (!address || !length) return;
  unsigned char encoded[1025];
  const unsigned char *bytes = address;
  size_t count = length < 512 ? length : 512;
  for (size_t index = 0; index < count; index++) {
    static const char digits[] = "0123456789abcdef";
    encoded[index * 2] = digits[bytes[index] >> 4];
    encoded[index * 2 + 1] = digits[bytes[index] & 0xf];
  }
  encoded[count * 2] = '\0';
  append_log(label, (const char *)encoded);
}

static void receive_probe(void *instance, void *message, int state) {
  int current = __atomic_add_fetch(&receive_probe_count, 1, __ATOMIC_RELAXED);
  if (current <= 200) {
    uintptr_t words[16] = {0};
    uintptr_t nested[16] = {0};
    if (message) memcpy(words, message, sizeof(words));
    if (words[0]) memcpy(nested, (const void *)words[0], sizeof(nested));
    char detail[512];
    snprintf(detail, sizeof(detail),
      "instance=%p message=%p state=%d words=%016lx,%016lx,%016lx,%016lx,%016lx,%016lx,%016lx,%016lx nested=%016lx,%016lx,%016lx,%016lx,%016lx,%016lx,%016lx,%016lx",
      instance, message, state, words[0], words[1], words[2], words[3], words[4], words[5], words[6], words[7],
      nested[0], nested[1], nested[2], nested[3], nested[4], nested[5], nested[6], nested[7]);
    append_log("receive probe", detail);
    append_hex_log("receive probe message bytes", message, 128);
    if (words[0]) {
      append_hex_log("receive probe nested bytes", (const void *)words[0], 128);
    }
  }
  if (receive_probe_original) receive_probe_original(instance, message, state);
}

static void install_receive_probe(void *wrapper) {
  if (!wrapper || __atomic_load_n(&receive_probe_installed, __ATOMIC_ACQUIRE)) return;
  struct link_map *map = NULL;
  if (dlinfo(wrapper, RTLD_DI_LINKMAP, &map) != 0 || !map) return;
  uintptr_t target = (uintptr_t)map->l_addr + 0x407b840;
  uint32_t *instructions = (uint32_t *)target;
  if (instructions[0] != 0xd10443ffU || instructions[1] != 0xa90c7bfdU
    || instructions[2] != 0xa90d67fcU || instructions[3] != 0xa90e5ff8U) return;
  long page_size = sysconf(_SC_PAGESIZE);
  if (page_size <= 0) return;
  uint8_t *trampoline = mmap(NULL, (size_t)page_size, PROT_READ | PROT_WRITE | PROT_EXEC,
    MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
  if (trampoline == MAP_FAILED) {
    append_log("receive probe unavailable", "trampoline mmap failed");
    return;
  }
  memcpy(trampoline, instructions, 16);
  uint32_t jump[] = { 0x58000050U, 0xd61f0200U };
  uintptr_t resume = target + 16;
  memcpy(trampoline + 16, jump, sizeof(jump));
  memcpy(trampoline + 24, &resume, sizeof(resume));
  receive_probe_original = (receive_probe_fn)trampoline;
  uintptr_t page = target & ~((uintptr_t)page_size - 1);
  if (mprotect((void *)page, (size_t)page_size, PROT_READ | PROT_WRITE | PROT_EXEC) != 0) {
    append_log("receive probe unavailable", "target mprotect failed");
    munmap(trampoline, (size_t)page_size);
    receive_probe_original = NULL;
    return;
  }
  uint8_t patch[16];
  memcpy(patch, jump, sizeof(jump));
  uintptr_t hook_address = (uintptr_t)receive_probe;
  memcpy(patch + 8, &hook_address, sizeof(hook_address));
  memcpy(instructions, patch, sizeof(patch));
  __builtin___clear_cache((char *)target, (char *)target + sizeof(patch));
  __builtin___clear_cache((char *)trampoline, (char *)trampoline + 32);
  mprotect((void *)page, (size_t)page_size, PROT_READ | PROT_EXEC);
  __atomic_store_n(&receive_probe_installed, 1, __ATOMIC_RELEASE);
  append_log("receive probe installed", "");
}

static void log_msf_state(void) {
  void *wrapper = dlopen("/opt/QQ/resources/app/wrapper.node", RTLD_NOLOAD | RTLD_LAZY);
  if (!wrapper) {
    append_log("msf state unavailable", "wrapper.node not loaded");
    return;
  }
  void *(*shared_instance)(void) = dlsym(wrapper, "_ZN3MSF6MSFSDK14sharedInstanceEv");
  int (*connect_state)(void *) = dlsym(wrapper, "_ZN3MSF6MSFSDK12connectStateEv");
  int (*is_connected)(void *) = dlsym(wrapper, "_ZN3MSF6MSFSDK11isConnectedEv");
  int (*current_conn_seq)(void *) = dlsym(wrapper, "_ZN3MSF6MSFSDK14currentConnSeqEv");
  if (!shared_instance) {
    append_log("msf state unavailable", "sharedInstance missing");
    return;
  }
  void *instance = shared_instance();
  int state = instance && connect_state ? connect_state(instance) : -1;
  int connected = instance && is_connected ? is_connected(instance) : -1;
  int sequence = instance && current_conn_seq ? current_conn_seq(instance) : -1;
  char detail[192];
  snprintf(detail, sizeof(detail), "instance=%p state=%d connected=%d sequence=%d", instance, state, connected, sequence);
  append_log("msf state", detail);
  const char *receive_probe_enabled = getenv("AMITIA_SNOWLUMA_RECEIVE_PROBE");
  if (receive_probe_enabled && strcmp(receive_probe_enabled, "1") == 0) install_receive_probe(wrapper);
}

static void append_log(const char *message, const char *detail) {
  const char *runtime = getenv("SNOWLUMA_HOOK_RUNTIME_DIR");
  char file[PATH_MAX];
  char line[4096];
  if (!runtime || runtime[0] != '/') return;
  mkdir(runtime, 0700);
  if (snprintf(file, sizeof(file), "%s/amitia-hook-loader.log", runtime) >= (int)sizeof(file)) return;
  int fd = open(file, O_WRONLY | O_CREAT | O_APPEND, 0600);
  if (fd < 0) return;
  int length = snprintf(line, sizeof(line), "%s%s%s\n", message, detail && detail[0] ? ": " : "", detail ? detail : "");
  if (length > 0) write(fd, line, (size_t)length < sizeof(line) ? (size_t)length : sizeof(line) - 1);
  close(fd);
}

static void *start_hook(void *opaque) {
  struct hook_start_request *request = opaque;
  char detail[PATH_MAX * 2];
  append_log("hook thread started", request->library);
  const char *delay_value = getenv("AMITIA_SNOWLUMA_HOOK_START_DELAY_MS");
  if (delay_value && delay_value[0]) {
    char *end = NULL;
    long delay_ms = strtol(delay_value, &end, 10);
    if (end && *end == '\0' && delay_ms > 0) {
      struct timespec delay = { .tv_sec = delay_ms / 1000, .tv_nsec = (delay_ms % 1000) * 1000000 };
      nanosleep(&delay, NULL);
    }
  }
  prctl(PR_SET_NAME, "qq", 0, 0, 0);
  void *handle = dlopen(request->library, RTLD_NOW | RTLD_GLOBAL);
  if (!handle) {
    const char *error = dlerror();
    append_log("dlopen failed", error ? error : "unknown error");
    free(request);
    return NULL;
  }
  verify_vm_fallback();
  dlerror();
  int (*start)(void) = (int (*)(void))dlsym(handle, "snowluma_linux_hook_start_dynamic");
  const char *error = dlerror();
  if (!start || error) {
    append_log("dlsym failed", error ? error : "start symbol missing");
    return NULL;
  }
  if (!start()) {
    append_log("hook start failed", request->library);
    return NULL;
  }
  snprintf(detail, sizeof(detail), "pid=%ld library=%s", (long)getpid(), request->library);
  append_log("hook started", detail);
  for (int attempt = 0; attempt < 3; attempt++) {
    struct timespec delay = { .tv_sec = attempt == 0 ? 5 : 5, .tv_nsec = 0 };
    nanosleep(&delay, NULL);
    log_msf_state();
  }
  free(request);
  return NULL;
}

static void request_hook_start(int signal_number) {
  (void)signal_number;
  hook_requested = 1;
}

static void launch_requested_hook(void) {
  if (!__atomic_exchange_n(&hook_launch_state, 1, __ATOMIC_ACQ_REL)) {
    append_log("launching requested hook", requested_hook_library);
    struct hook_start_request *request = calloc(1, sizeof(*request));
    if (!request) {
      __atomic_store_n(&hook_launch_state, 0, __ATOMIC_RELEASE);
      append_log("allocation failed", "");
      return;
    }
    snprintf(request->library, sizeof(request->library), "%s", requested_hook_library);
    pthread_t thread;
    if (pthread_create(&thread, NULL, start_hook, request) != 0) {
      free(request);
      __atomic_store_n(&hook_launch_state, 0, __ATOMIC_RELEASE);
      append_log("thread creation failed", "");
      return;
    }
    pthread_detach(thread);
  }
}

static void *control_hook_start(void *opaque) {
  (void)opaque;
  for (;;) {
    if (hook_requested) {
      hook_requested = 0;
      append_log("hook request received", "");
      if (__atomic_load_n(&hook_launch_state, __ATOMIC_ACQUIRE)) {
        append_log("hook refresh ignored", "in-process restart is disabled");
      } else {
        launch_requested_hook();
      }
    }
    usleep(200000);
  }
  return NULL;
}

static int has_process_type(char *const argv[]) {
  if (!argv) return 0;
  for (size_t index = 1; argv[index]; index++) {
    if (strncmp(argv[index], "--type=", 7) == 0) return 1;
  }
  return 0;
}

int prctl(int option, ...) {
  va_list arguments;
  va_start(arguments, option);
  unsigned long first = va_arg(arguments, unsigned long);
  unsigned long second = va_arg(arguments, unsigned long);
  unsigned long third = va_arg(arguments, unsigned long);
  unsigned long fourth = va_arg(arguments, unsigned long);
  va_end(arguments);
  if (option == PR_SET_NAME && syscall(SYS_gettid) == getpid()) {
    first = (unsigned long)"qq";
    second = 0;
    third = 0;
    fourth = 0;
  }
  return (int)syscall(SYS_prctl, option, first, second, third, fourth);
}

static const char *redirect_main_path(const char *path, char *const argv[]) {
  const char *real = getenv("AMITIA_SNOWLUMA_QQ_REAL_PATH");
  const char *wrapper = getenv("AMITIA_SNOWLUMA_QQ_WRAPPER_PATH");
  if (!path || !real || !wrapper || has_process_type(argv)) return path;
  if (strcmp(path, real) == 0) return wrapper;
  const char *base = strrchr(path, '/');
  base = base ? base + 1 : path;
  return strcmp(base, "qq.amitia-real") == 0 ? wrapper : path;
}

static void *next_symbol(const char *name) {
  return dlsym(RTLD_NEXT, name);
}

static uint16_t read_u16(const unsigned char *buffer, size_t offset) {
  return (uint16_t)buffer[offset] | ((uint16_t)buffer[offset + 1] << 8);
}

static uint32_t read_u32(const unsigned char *buffer, size_t offset) {
  return (uint32_t)buffer[offset]
    | ((uint32_t)buffer[offset + 1] << 8)
    | ((uint32_t)buffer[offset + 2] << 16)
    | ((uint32_t)buffer[offset + 3] << 24);
}

static uint64_t read_u64(const unsigned char *buffer, size_t offset) {
  uint64_t value = 0;
  for (size_t index = 0; index < 8; index++) value |= (uint64_t)buffer[offset + index] << (index * 8);
  return value;
}

static void write_u16(unsigned char *buffer, size_t offset, uint16_t value) {
  buffer[offset] = (unsigned char)value;
  buffer[offset + 1] = (unsigned char)(value >> 8);
}

static void write_u32(unsigned char *buffer, size_t offset, uint32_t value) {
  for (size_t index = 0; index < 4; index++) buffer[offset + index] = (unsigned char)(value >> (index * 8));
}

static void write_u64(unsigned char *buffer, size_t offset, uint64_t value) {
  for (size_t index = 0; index < 8; index++) buffer[offset + index] = (unsigned char)(value >> (index * 8));
}

static int send_login_state(int socket, uint64_t uin) {
  unsigned char frame[40] = {0};
  write_u32(frame, 0, 0x31504851U);
  write_u16(frame, 4, 1);
  write_u16(frame, 6, 7);
  write_u32(frame, 12, 1);
  write_u32(frame, 16, 4);
  write_u64(frame, 32, uin);
  ssize_t (*next)(int, const void *, size_t, int) = next_symbol("send");
  if (!next) {
    errno = ENOSYS;
    return -1;
  }
  ssize_t result = next(socket, frame, sizeof(frame), MSG_NOSIGNAL);
  char detail[96];
  snprintf(detail, sizeof(detail), "uin=%llu result=%ld errno=%d", (unsigned long long)uin, (long)result, result < 0 ? errno : 0);
  append_log("login state bridge", detail);
  return result == (ssize_t)sizeof(frame) ? 0 : -1;
}

static void *send_login_state_after_delay(void *opaque) {
  int socket = (int)(intptr_t)opaque;
  const char *delay_value = getenv("AMITIA_SNOWLUMA_LOGIN_STATE_DELAY_MS");
  long delay_ms = delay_value && delay_value[0] ? strtol(delay_value, NULL, 10) : 5000;
  if (delay_ms > 0) {
    struct timespec delay = { .tv_sec = delay_ms / 1000, .tv_nsec = (delay_ms % 1000) * 1000000 };
    nanosleep(&delay, NULL);
  }
  uint64_t uin = __atomic_load_n(&bridge_login_uin, __ATOMIC_ACQUIRE);
  if (uin > 0) send_login_state(socket, uin);
  __atomic_store_n(&bridge_login_state_launching, 0, __ATOMIC_RELEASE);
  return NULL;
}

__asm__(".symver amitia_recv,recv@@GLIBC_2.17");

ssize_t amitia_recv(int socket, void *buffer, size_t length, int flags) {
  ssize_t (*next)(int, void *, size_t, int) = next_symbol("recv");
  if (!next) {
    errno = ENOSYS;
    return -1;
  }
  ssize_t result = next(socket, buffer, length, flags);
  int saved = errno;
  const char *enabled = getenv("AMITIA_SNOWLUMA_PIPE_BRIDGE");
  if (result >= 40 && enabled && strcmp(enabled, "1") == 0) {
    const unsigned char *frame = buffer;
    if (read_u32(frame, 0) == 0x31504851U && read_u16(frame, 4) == 1 && read_u16(frame, 6) == 17) {
      uint64_t uin = read_u64(frame, 32);
      if (uin > 0) {
        __atomic_store_n(&bridge_login_uin, uin, __ATOMIC_RELEASE);
        if (!__atomic_exchange_n(&bridge_login_state_launching, 1, __ATOMIC_ACQ_REL)) {
          pthread_t thread;
          if (pthread_create(&thread, NULL, send_login_state_after_delay, (void *)(intptr_t)socket) == 0) {
            pthread_detach(thread);
            append_log("login identity hint received", "scheduled delayed login state");
          } else {
            __atomic_store_n(&bridge_login_state_launching, 0, __ATOMIC_RELEASE);
          }
        }
      }
    }
  }
  errno = saved;
  return result;
}

__asm__(".symver amitia_send,send@@GLIBC_2.17");

ssize_t amitia_send(int socket, const void *buffer, size_t length, int flags) {
  ssize_t (*next)(int, const void *, size_t, int) = next_symbol("send");
  if (!next) {
    errno = ENOSYS;
    return -1;
  }
  const void *outgoing = buffer;
  unsigned char rewritten[1024];
  const char *enabled = getenv("AMITIA_SNOWLUMA_PIPE_BRIDGE");
  if (length >= 40 && length <= sizeof(rewritten) && enabled && strcmp(enabled, "1") == 0) {
    const unsigned char *frame = buffer;
    uint64_t uin = __atomic_load_n(&bridge_login_uin, __ATOMIC_ACQUIRE);
    if (read_u32(frame, 0) == 0x31504851U && read_u16(frame, 4) == 1
      && read_u16(frame, 6) == 7 && (int32_t)read_u32(frame, 12) == 0 && uin > 0) {
      memcpy(rewritten, buffer, length);
      write_u32(rewritten, 12, 1);
      write_u32(rewritten, 16, 4);
      write_u64(rewritten, 32, uin);
      outgoing = rewritten;
      append_log("login state guard", "rewrote native logout frame");
    }
  }
  errno = 0;
  ssize_t result = next(socket, outgoing, length, flags);
  int saved = errno;
  if (length >= 40 && result >= 40) {
    const unsigned char *frame = buffer;
    if (read_u32(frame, 0) == 0x31504851U && read_u16(frame, 4) == 1) {
      char detail[96];
      snprintf(detail, sizeof(detail), "op=%u status=%d result=%ld", read_u16(frame, 6), (int)read_u32(frame, 12), (long)result);
      append_log("hook frame sent", detail);
    }
  }
  errno = saved;
  return result;
}

__asm__(".symver amitia_ptrace,ptrace@@GLIBC_2.17");

long amitia_ptrace(enum __ptrace_request request, ...) {
  static pid_t attached = 0;
  long (*next)(enum __ptrace_request, ...) = next_symbol("ptrace");
  if (!next) { errno = ENOSYS; return -1; }
  va_list arguments;
  va_start(arguments, request);
  pid_t pid = va_arg(arguments, pid_t);
  void *address = va_arg(arguments, void *);
  void *data = va_arg(arguments, void *);
  va_end(arguments);
  errno = 0;
  const char *enabled = getenv("AMITIA_PTRACE_SEIZE_COMPAT");
  if (enabled && strcmp(enabled, "1") == 0) {
    if (request == PTRACE_SEIZE && !address && !data) {
      long result = next(PTRACE_ATTACH, pid, NULL, NULL);
      int saved = errno;
      if (result == 0) {
        attached = pid;
        append_log("ptrace seize compatibility", "attached through virtual PTRACE_ATTACH");
      }
      char detail[256];
      snprintf(detail, sizeof(detail), "request=SEIZE->ATTACH pid=%ld address=%p data=%p result=%ld errno=%d", (long)pid, address, data, result, saved);
      append_log("ptrace operation", detail);
      errno = saved;
      return result;
    }
    if (request == PTRACE_INTERRUPT && attached == pid) {
      char detail[256];
      snprintf(detail, sizeof(detail), "request=INTERRUPT pid=%ld result=0 errno=0", (long)pid);
      append_log("ptrace operation", detail);
      errno = 0;
      return 0;
    }
  }
  long result = next(request, pid, address, data);
  int saved = errno;
  if (request != PTRACE_PEEKDATA && request != PTRACE_POKEDATA) {
    char detail[256];
    snprintf(detail, sizeof(detail), "request=0x%x pid=%ld address=%p data=%p result=%ld errno=%d", (unsigned int)request, (long)pid, address, data, result, saved);
    append_log("ptrace operation", detail);
  }
  if (request == PTRACE_DETACH && result == 0 && attached == pid) attached = 0;
  errno = saved;
  return result;
}

pid_t waitpid(pid_t pid, int *status, int options) {
  pid_t (*next)(pid_t, int *, int) = next_symbol("waitpid");
  if (!next) { errno = ENOSYS; return -1; }
  errno = 0;
  pid_t result = next(pid, status, options);
  int saved = errno;
  char detail[256];
  snprintf(detail, sizeof(detail), "pid=%ld result=%ld status=0x%x options=0x%x errno=%d", (long)pid, (long)result, status ? (unsigned int)*status : 0, options, saved);
  append_log("waitpid", detail);
  errno = saved;
  return result;
}

static const char *rewrite_qq_path(const char *path) {
  if (!path || path[0] != '/') return path;
  const char *marker = strstr(path, "/opt/QQ/");
  return marker ? marker : path;
}

FILE *fopen(const char *path, const char *mode) {
  static int logged = 0;
  FILE *(*next)(const char *, const char *) = next_symbol("fopen");
  if (!next) { errno = ENOSYS; return NULL; }
  const char *resolved = rewrite_qq_path(path);
  FILE *result = next(resolved, mode);
  int saved = errno;
  if (resolved != path && strstr(resolved, "wrapper.node") && !__atomic_exchange_n(&logged, 1, __ATOMIC_RELAXED)) append_log(result ? "wrapper opened" : "wrapper open failed", resolved);
  errno = saved;
  return result;
}

FILE *fopen64(const char *path, const char *mode) {
  static int logged = 0;
  FILE *(*next)(const char *, const char *) = next_symbol("fopen64");
  if (!next) { errno = ENOSYS; return NULL; }
  const char *resolved = rewrite_qq_path(path);
  FILE *result = next(resolved, mode);
  int saved = errno;
  if (resolved != path && strstr(resolved, "wrapper.node") && !__atomic_exchange_n(&logged, 1, __ATOMIC_RELAXED)) append_log(result ? "wrapper opened" : "wrapper open failed", resolved);
  errno = saved;
  return result;
}

static mode_t open_mode(int flags, va_list arguments) {
  return flags & (O_CREAT | O_TMPFILE) ? va_arg(arguments, mode_t) : 0;
}

int open(const char *path, int flags, ...) {
  va_list arguments;
  va_start(arguments, flags);
  mode_t mode = open_mode(flags, arguments);
  va_end(arguments);
  return (int)syscall(SYS_openat, AT_FDCWD, rewrite_qq_path(path), flags, mode);
}

int open64(const char *path, int flags, ...) {
  va_list arguments;
  va_start(arguments, flags);
  mode_t mode = open_mode(flags, arguments);
  va_end(arguments);
  return (int)syscall(SYS_openat, AT_FDCWD, rewrite_qq_path(path), flags, mode);
}

int openat(int directory, const char *path, int flags, ...) {
  va_list arguments;
  va_start(arguments, flags);
  mode_t mode = open_mode(flags, arguments);
  va_end(arguments);
  return (int)syscall(SYS_openat, directory, rewrite_qq_path(path), flags, mode);
}

int openat64(int directory, const char *path, int flags, ...) {
  va_list arguments;
  va_start(arguments, flags);
  mode_t mode = open_mode(flags, arguments);
  va_end(arguments);
  return (int)syscall(SYS_openat, directory, rewrite_qq_path(path), flags, mode);
}

static int prefer_proc_mem(void) {
  const char *value = getenv("AMITIA_SNOWLUMA_PROC_MEM");
  return value && strcmp(value, "1") == 0;
}

static void log_memory_failure(pid_t pid, int writing, int error, const char *stage) {
  static int failures = 0;
  if (__atomic_add_fetch(&failures, 1, __ATOMIC_RELAXED) > 12) return;
  char detail[256];
  snprintf(detail, sizeof(detail), "pid=%ld op=%s stage=%s errno=%d", (long)pid, writing ? "write" : "read", stage, error);
  append_log("process_vm failure", detail);
}

static ssize_t proc_mem_transfer(pid_t pid, const struct iovec *local_iov, unsigned long local_count, const struct iovec *remote_iov, unsigned long remote_count, int writing) {
  static int call_count = 0;
  char file[64];
  size_t local_index = 0, remote_index = 0, local_offset = 0, remote_offset = 0;
  ssize_t total = 0;
  int saved = 0;
  if (snprintf(file, sizeof(file), "/proc/%ld/mem", (long)pid) >= (int)sizeof(file)) {
    errno = EINVAL;
    return -1;
  }
  int fd = open(file, writing ? O_RDWR | O_CLOEXEC : O_RDONLY | O_CLOEXEC);
  if (fd < 0) {
    int error = errno;
    log_memory_failure(pid, writing, error, "open");
    errno = error;
    return -1;
  }
  while (local_index < local_count && remote_index < remote_count) {
    const struct iovec *local = &local_iov[local_index];
    const struct iovec *remote = &remote_iov[remote_index];
    size_t local_remaining = local->iov_len - local_offset;
    size_t remote_remaining = remote->iov_len - remote_offset;
    size_t length = local_remaining < remote_remaining ? local_remaining : remote_remaining;
    char *local_address = (char *)local->iov_base + local_offset;
    char *remote_address = (char *)remote->iov_base + remote_offset;
    ssize_t copied;
    if (!length) {
      if (!local_remaining) {
        local_index++;
        local_offset = 0;
      }
      if (!remote_remaining) {
        remote_index++;
        remote_offset = 0;
      }
      continue;
    }
    copied = writing
      ? pwrite(fd, local_address, length, (off_t)(uintptr_t)remote_address)
      : pread(fd, local_address, length, (off_t)(uintptr_t)remote_address);
    if (copied < 0) {
      saved = errno;
      log_memory_failure(pid, writing, saved, "transfer");
      close(fd);
      errno = saved;
      return -1;
    }
    total += copied;
    local_offset += (size_t)copied;
    remote_offset += (size_t)copied;
    if (copied == 0 || (size_t)copied < length) break;
    if (local_offset == local->iov_len) {
      local_index++;
      local_offset = 0;
    }
    if (remote_offset == remote->iov_len) {
      remote_index++;
      remote_offset = 0;
    }
  }
  close(fd);
  int call = __atomic_add_fetch(&call_count, 1, __ATOMIC_RELAXED);
  if (call <= 24) {
    char detail[256];
    snprintf(detail, sizeof(detail), "call=%d op=%s pid=%ld local=%lu remote=%lu bytes=%ld error=%d", call, writing ? "write" : "read", (long)pid, local_count, remote_count, (long)total, total < 0 ? saved : 0);
    append_log("process_vm transfer", detail);
  }
  if (total == 0) {
    errno = EFAULT;
    return -1;
  }
  return total;
}

static void log_vm_transfer(const char *operation, int first_error, ssize_t result, int fallback_error) {
  static int read_logged = 0;
  static int write_logged = 0;
  int *logged = strcmp(operation, "read") == 0 ? &read_logged : &write_logged;
  char detail[512];
  if (!__atomic_exchange_n(logged, 1, __ATOMIC_RELAXED)) {
    snprintf(detail, sizeof(detail), "first_errno=%d result=%ld fallback_errno=%d", first_error, (long)result, fallback_error);
    append_log(operation, detail);
  }
}

__asm__(".symver amitia_process_vm_readv,process_vm_readv@@GLIBC_2.17");

ssize_t amitia_process_vm_readv(pid_t pid, const struct iovec *local_iov, unsigned long local_count, const struct iovec *remote_iov, unsigned long remote_count, unsigned long flags) {
  typedef ssize_t (*process_vm_fn)(pid_t, const struct iovec *, unsigned long, const struct iovec *, unsigned long, unsigned long);
  process_vm_fn next = (process_vm_fn)next_symbol("process_vm_readv");
  int first_error = 0;
  if (next && !prefer_proc_mem()) {
    ssize_t result = next(pid, local_iov, local_count, remote_iov, remote_count, flags);
    if (result >= 0 || (errno != ENOSYS && errno != EPERM)) {
      log_vm_transfer("read", 0, result, result < 0 ? errno : 0);
      return result;
    }
    first_error = errno;
  }
  if (flags != 0) {
    errno = EINVAL;
    return -1;
  }
  ssize_t result = proc_mem_transfer(pid, local_iov, local_count, remote_iov, remote_count, 0);
  log_vm_transfer("read", first_error, result, result < 0 ? errno : 0);
  return result;
}

__asm__(".symver amitia_process_vm_writev,process_vm_writev@@GLIBC_2.17");

ssize_t amitia_process_vm_writev(pid_t pid, const struct iovec *local_iov, unsigned long local_count, const struct iovec *remote_iov, unsigned long remote_count, unsigned long flags) {
  typedef ssize_t (*process_vm_fn)(pid_t, const struct iovec *, unsigned long, const struct iovec *, unsigned long, unsigned long);
  process_vm_fn next = (process_vm_fn)next_symbol("process_vm_writev");
  int first_error = 0;
  if (next && !prefer_proc_mem()) {
    ssize_t result = next(pid, local_iov, local_count, remote_iov, remote_count, flags);
    if (result >= 0 || (errno != ENOSYS && errno != EPERM)) {
      log_vm_transfer("write", 0, result, result < 0 ? errno : 0);
      return result;
    }
    first_error = errno;
  }
  if (flags != 0) {
    errno = EINVAL;
    return -1;
  }
  ssize_t result = proc_mem_transfer(pid, local_iov, local_count, remote_iov, remote_count, 1);
  log_vm_transfer("write", first_error, result, result < 0 ? errno : 0);
  return result;
}

static int page_protection(uintptr_t address) {
  FILE *maps = fopen("/proc/self/maps", "r");
  if (!maps) return -1;
  char line[512];
  int protection = -1;
  while (fgets(line, sizeof(line), maps)) {
    unsigned long start = 0, end = 0;
    char permissions[5] = "";
    if (sscanf(line, "%lx-%lx %4s", &start, &end, permissions) != 3) continue;
    if (address < start || address >= end) continue;
    protection = 0;
    if (permissions[0] == 'r') protection |= PROT_READ;
    if (permissions[1] == 'w') protection |= PROT_WRITE;
    if (permissions[2] == 'x') protection |= PROT_EXEC;
    break;
  }
  fclose(maps);
  return protection;
}

static int patch_vm_relocation(uintptr_t *slot, uintptr_t replacement) {
  long page_size = sysconf(_SC_PAGESIZE);
  if (page_size <= 0) return 0;
  uintptr_t start = (uintptr_t)slot & ~((uintptr_t)page_size - 1);
  uintptr_t end = ((uintptr_t)slot + sizeof(*slot) + (uintptr_t)page_size - 1) & ~((uintptr_t)page_size - 1);
  int protection = page_protection((uintptr_t)slot);
  if (protection < 0 || mprotect((void *)start, end - start, protection | PROT_WRITE) != 0) return 0;
  __atomic_store_n(slot, replacement, __ATOMIC_RELEASE);
  __builtin___clear_cache((char *)slot, (char *)slot + sizeof(*slot));
  mprotect((void *)start, end - start, protection);
  return 1;
}

static int redirect_vm_symbols(void *handle, const char *name, uintptr_t replacement) {
  struct link_map *map = NULL;
  if (!handle || dlinfo(handle, RTLD_DI_LINKMAP, &map) != 0 || !map || !map->l_ld) return 0;
  Elf64_Sym *symbols = NULL;
  const char *strings = NULL;
  Elf64_Rela *relocations = NULL;
  size_t relocation_size = 0;
  for (Elf64_Dyn *entry = (Elf64_Dyn *)map->l_ld; entry->d_tag != DT_NULL; entry++) {
    switch (entry->d_tag) {
      case DT_SYMTAB:
        symbols = (Elf64_Sym *)entry->d_un.d_ptr;
        break;
      case DT_STRTAB:
        strings = (const char *)entry->d_un.d_ptr;
        break;
      case DT_JMPREL:
        relocations = (Elf64_Rela *)entry->d_un.d_ptr;
        break;
      case DT_PLTRELSZ:
        relocation_size = entry->d_un.d_val;
        break;
    }
  }
  if (!symbols || !strings || !relocations || !relocation_size) return 0;
  int patched = 0;
  size_t count = relocation_size / sizeof(Elf64_Rela);
  for (size_t index = 0; index < count; index++) {
    Elf64_Rela *relocation = &relocations[index];
    unsigned int type = ELF64_R_TYPE(relocation->r_info);
    unsigned int symbol_index = ELF64_R_SYM(relocation->r_info);
    if (type != R_AARCH64_JUMP_SLOT && type != R_AARCH64_GLOB_DAT) continue;
    const char *symbol_name = strings + symbols[symbol_index].st_name;
    if (strcmp(symbol_name, name) != 0) continue;
    uintptr_t *slot = (uintptr_t *)(map->l_addr + relocation->r_offset);
    if (patch_vm_relocation(slot, replacement)) patched++;
  }
  return patched;
}

void *dlopen(const char *file, int flags) {
  static void *(*next)(const char *, int) = NULL;
  if (!next) next = (void *(*)(const char *, int))next_symbol("dlopen");
  if (!next) return NULL;
  void *handle = next(file, flags);
  if (!handle) return NULL;
  const char *start_on_wrapper = getenv("AMITIA_SNOWLUMA_HOOK_START_ON_WRAPPER");
  if (start_on_wrapper && strcmp(start_on_wrapper, "1") == 0
    && file && strstr(file, "wrapper.node") && requested_hook_library[0]) {
    launch_requested_hook();
  }
  int read_slots = redirect_vm_symbols(handle, "process_vm_readv", (uintptr_t)amitia_process_vm_readv);
  int write_slots = redirect_vm_symbols(handle, "process_vm_writev", (uintptr_t)amitia_process_vm_writev);
  int ptrace_slots = redirect_vm_symbols(handle, "ptrace", (uintptr_t)amitia_ptrace);
  int send_slots = redirect_vm_symbols(handle, "send", (uintptr_t)amitia_send);
  int recv_slots = redirect_vm_symbols(handle, "recv", (uintptr_t)amitia_recv);
  if (read_slots || write_slots || ptrace_slots || send_slots || recv_slots) {
    char detail[PATH_MAX + 128];
    snprintf(detail, sizeof(detail), "file=%s read=%d write=%d ptrace=%d send=%d recv=%d", file ? file : "", read_slots, write_slots, ptrace_slots, send_slots, recv_slots);
    append_log("process_vm relocations", detail);
  }
  return handle;
}

static void verify_vm_fallback(void) {
  uintptr_t probe = 0x534e4f574c554d41ULL;
  uintptr_t copied = 0;
  struct iovec local = { .iov_base = &copied, .iov_len = sizeof(copied) };
  struct iovec remote = { .iov_base = &probe, .iov_len = sizeof(probe) };
  void *preferred = dlsym(RTLD_DEFAULT, "process_vm_readv");
  errno = 0;
  ssize_t result = amitia_process_vm_readv(getpid(), &local, 1, &remote, 1, 0);
  int saved = errno;
  char detail[256];
  snprintf(detail, sizeof(detail), "preferred=%s result=%ld verified=%d errno=%d", preferred == (void *)amitia_process_vm_readv ? "loader" : "libc", (long)result, result == sizeof(copied) && copied == probe, saved);
  append_log("process_vm_readv self-test", detail);
}

int execve(const char *path, char *const argv[], char *const envp[]) {
  path = redirect_main_path(path, argv);
  return (int)syscall(SYS_execve, path, argv, envp);
}

int execv(const char *path, char *const argv[]) {
  return (int)syscall(SYS_execve, redirect_main_path(path, argv), argv, environ);
}

int execvp(const char *file, char *const argv[]) {
  if (redirect_main_path(file, argv) != file) {
    return (int)syscall(SYS_execve, redirect_main_path(file, argv), argv, environ);
  }
  int (*next)(const char *, char *const[]) = next_symbol("execvp");
  return next ? next(file, argv) : -1;
}

int execvpe(const char *file, char *const argv[], char *const envp[]) {
  if (redirect_main_path(file, argv) != file) {
    return (int)syscall(SYS_execve, redirect_main_path(file, argv), argv, envp);
  }
  int (*next)(const char *, char *const[], char *const[]) = next_symbol("execvpe");
  return next ? next(file, argv, envp) : -1;
}

int posix_spawn(pid_t *pid, const char *path, const posix_spawn_file_actions_t *actions, const posix_spawnattr_t *attributes, char *const argv[], char *const envp[]) {
  int (*next)(pid_t *, const char *, const posix_spawn_file_actions_t *, const posix_spawnattr_t *, char *const[], char *const[]) = next_symbol("posix_spawn");
  return next ? next(pid, redirect_main_path(path, argv), actions, attributes, argv, envp) : -1;
}

int posix_spawnp(pid_t *pid, const char *file, const posix_spawn_file_actions_t *actions, const posix_spawnattr_t *attributes, char *const argv[], char *const envp[]) {
  int (*next)(pid_t *, const char *, const posix_spawn_file_actions_t *, const posix_spawnattr_t *, char *const[], char *const[]) = next_symbol("posix_spawnp");
  return next ? next(pid, redirect_main_path(file, argv), actions, attributes, argv, envp) : -1;
}

__attribute__((constructor)) static void prepare_snowluma_hook(void) {
  const char *library = getenv("AMITIA_SNOWLUMA_HOOK_LIBRARY");
  if (!library || library[0] != '/') return;
  prctl(PR_SET_NAME, "qq", 0, 0, 0);
  snprintf(requested_hook_library, sizeof(requested_hook_library), "%s", library);
  unsetenv("AMITIA_SNOWLUMA_HOOK_LIBRARY");
  unsetenv("LD_PRELOAD");
  if (!getenv("SNOWLUMA_HOOK_RUNTIME_DIR")) {
    char runtime[128];
    snprintf(runtime, sizeof(runtime), "/tmp/snowluma-%u", (unsigned int)getuid());
    setenv("SNOWLUMA_HOOK_RUNTIME_DIR", runtime, 1);
  }
  struct sigaction action;
  memset(&action, 0, sizeof(action));
  action.sa_handler = request_hook_start;
  sigemptyset(&action.sa_mask);
  action.sa_flags = SA_RESTART;
  if (sigaction(SIGUSR1, &action, NULL) != 0) {
    append_log("signal handler failed", strerror(errno));
    return;
  }
  pthread_t thread;
  if (pthread_create(&thread, NULL, control_hook_start, NULL) != 0) {
    append_log("thread creation failed", "");
    return;
  }
  pthread_detach(thread);
  const char *autostart = getenv("AMITIA_SNOWLUMA_HOOK_AUTOSTART");
  if (!autostart || strcmp(autostart, "1") == 0) hook_requested = 1;
}
