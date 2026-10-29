#define _GNU_SOURCE

#include <dlfcn.h>
#include <fcntl.h>
#include <limits.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

struct hook_start_request {
  char library[PATH_MAX];
};

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
  sleep(4);
  void *handle = dlopen(request->library, RTLD_NOW | RTLD_GLOBAL);
  if (!handle) {
    const char *error = dlerror();
    append_log("dlopen failed", error ? error : "unknown error");
    free(request);
    return NULL;
  }
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
  free(request);
  return NULL;
}

__attribute__((constructor)) static void prepare_snowluma_hook(void) {
  const char *library = getenv("AMITIA_SNOWLUMA_HOOK_LIBRARY");
  if (!library || library[0] != '/') return;
  const char *expectedParent = getenv("AMITIA_SNOWLUMA_HOOK_PARENT_PID");
  char parent[32];
  snprintf(parent, sizeof(parent), "%ld", (long)getppid());
  if (!expectedParent || strcmp(expectedParent, parent) != 0) return;
  unsetenv("AMITIA_SNOWLUMA_HOOK_PARENT_PID");
  unsetenv("AMITIA_SNOWLUMA_HOOK_LIBRARY");
  unsetenv("LD_PRELOAD");
  if (!getenv("SNOWLUMA_HOOK_RUNTIME_DIR")) {
    char runtime[128];
    snprintf(runtime, sizeof(runtime), "/tmp/snowluma-%u", (unsigned int)getuid());
    setenv("SNOWLUMA_HOOK_RUNTIME_DIR", runtime, 1);
  }
  struct hook_start_request *request = calloc(1, sizeof(*request));
  if (!request) {
    append_log("allocation failed", "");
    return;
  }
  snprintf(request->library, sizeof(request->library), "%s", library);
  pthread_t thread;
  if (pthread_create(&thread, NULL, start_hook, request) != 0) {
    append_log("thread creation failed", "");
    free(request);
    return;
  }
  pthread_detach(thread);
}
