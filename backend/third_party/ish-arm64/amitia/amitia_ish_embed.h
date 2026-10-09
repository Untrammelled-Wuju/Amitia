#ifndef AMITIA_ISH_EMBED_H
#define AMITIA_ISH_EMBED_H

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum {
    AMITIA_ISH_UNAVAILABLE = 0,
    AMITIA_ISH_STARTING,
    AMITIA_ISH_RUNNING,
    AMITIA_ISH_ERROR
} amitia_ish_state_t;

typedef enum {
    AMITIA_ISH_OK = 0,
    AMITIA_ISH_ERR_NOT_INITIALIZED = -1,
    AMITIA_ISH_ERR_ROOTFS_NOT_READY = -2,
    AMITIA_ISH_ERR_ROOTFS_FORMAT_UNSUPPORTED = -3,
    AMITIA_ISH_ERR_INVALID_ARGUMENT = -4,
    AMITIA_ISH_ERR_EXEC_FAILED = -5,
    AMITIA_ISH_ERR_EXEC_TIMEOUT = -6,
    AMITIA_ISH_ERR_EXEC_CANCELLED = -7,
    AMITIA_ISH_ERR_EXEC_BUSY = -8,
    AMITIA_ISH_ERR_INTERNAL = -9
} amitia_ish_error_t;

typedef struct {
    int argc;
    const char *const *argv;
    const void *stdin_data;
    size_t stdin_size;
    const char *workdir;
    uint32_t timeout_ms;
    const char *execution_id;
    const char *const *env;
    size_t env_count;
    bool enable_host_bridge;
} amitia_ish_command_t;

typedef struct {
    int exit_code;
    const char *stdout_data;
    size_t stdout_size;
    const char *stderr_data;
    size_t stderr_size;
    amitia_ish_error_t error_code;
    char *error_message;
    uint64_t generation;
    bool fatal;
} amitia_ish_result_t;

typedef struct {
    uint64_t generation;
    int stdin_fd;
    int stdout_fd;
    int stderr_fd;
    int host_bridge_fd;
} amitia_ish_process_t;

int amitia_ish_start(const char *rootfs_path, const char *workdir, const char **env, size_t env_count);
int amitia_ish_execute(const amitia_ish_command_t *command, amitia_ish_result_t *result);
int amitia_ish_spawn(const amitia_ish_command_t *command, amitia_ish_process_t *process);
int amitia_ish_poll(uint64_t generation, bool *running, int *exit_code);
int amitia_ish_reap(uint64_t generation, int *exit_code);
int amitia_ish_cancel(uint64_t generation);
void amitia_ish_stop(void);
amitia_ish_state_t amitia_ish_state(void);
void amitia_ish_result_free(amitia_ish_result_t *result);

#ifdef __cplusplus
}
#endif
#endif
