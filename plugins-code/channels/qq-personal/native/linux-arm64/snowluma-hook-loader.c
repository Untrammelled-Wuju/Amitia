#define _GNU_SOURCE

#include <dlfcn.h>
#include <fcntl.h>
#include <limits.h>
#include <pthread.h>
#include <spawn.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/prctl.h>
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
  prctl(PR_SET_NAME, "qq", 0, 0, 0);
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

static void *maintain_process_name(void *opaque) {
  (void)opaque;
  for (;;) {
    prctl(PR_SET_NAME, "qq", 0, 0, 0);
    sleep(1);
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
  if (option == PR_SET_NAME) {
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
  pthread_t name_thread;
  if (pthread_create(&name_thread, NULL, maintain_process_name, NULL) == 0) pthread_detach(name_thread);
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
