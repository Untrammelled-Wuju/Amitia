#include "amitia_ish_embed.h"

#include <errno.h>
#include <dirent.h>
#include <fcntl.h>
#include <poll.h>
#include <pthread.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <time.h>
#include <unistd.h>

#include "kernel/init.h"
#include "kernel/task.h"
#include "kernel/calls.h"
#include "fs/fd.h"
#include "fs/real.h"
#include "fs/devices.h"

#define AMITIA_OUTPUT_LIMIT (8 * 1024 * 1024)
#define AMITIA_ARGUMENT_LIMIT (128 * 1024)
#define AMITIA_PROCESS_LIMIT 64
#define AMITIA_WNOHANG 1

struct amitia_process {
    uint64_t generation;
    pid_t_ pid;
    bool completed;
    bool cancelled;
    int exit_code;
    struct amitia_process *next;
};

static pthread_mutex_t runtime_lock = PTHREAD_MUTEX_INITIALIZER;
static pthread_mutex_t short_execution_lock = PTHREAD_MUTEX_INITIALIZER;
static _Atomic int runtime_state = AMITIA_ISH_UNAVAILABLE;
static struct task *init_task;
static char *mounted_root;
static char *mounted_business_volume;
static char *runtime_environment;
static bool kernel_ready;
static struct amitia_process *processes;
static uint64_t next_generation = 1;
static void (*previous_exit_hook)(struct task *, int);
static struct amitia_process *find_process(uint64_t generation);

struct amitia_bridge_fd {
    uint64_t generation;
    pid_t_ owner;
};

static bool bridge_authorized(struct fd *fd) {
    struct amitia_bridge_fd *bridge = fd->fs_data;
    if (!current || !bridge || current->tgid != bridge->owner) return false;
    lock(&pids_lock);
    struct amitia_process *process = find_process(bridge->generation);
    bool permitted = process && process->pid == bridge->owner && !process->completed && !process->cancelled;
    unlock(&pids_lock);
    return permitted;
}

static ssize_t bridge_read(struct fd *fd, void *bytes, size_t count) {
    return bridge_authorized(fd) ? realfs_fdops.read(fd, bytes, count) : _EACCES;
}

static ssize_t bridge_write(struct fd *fd, const void *bytes, size_t count) {
    return bridge_authorized(fd) ? realfs_fdops.write(fd, bytes, count) : _EACCES;
}

static int bridge_poll(struct fd *fd) {
    return bridge_authorized(fd) ? realfs_fdops.poll(fd) : 0;
}

static int bridge_close(struct fd *fd) {
    free(fd->fs_data);
    return realfs_fdops.close(fd);
}

static struct fd_ops bridge_operations;

static const char *default_environment[] = {
    "HOME=/root", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "TERM=xterm-256color", "NO_COLOR=1"
};

static uint64_t monotonic_ms(void) {
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (uint64_t) now.tv_sec * 1000 + now.tv_nsec / 1000000;
}

static struct amitia_process *find_process(uint64_t generation) {
    for (struct amitia_process *item = processes; item; item = item->next)
        if (item->generation == generation) return item;
    return NULL;
}

static int install_bridge_fd(int descriptor, pid_t_ owner, uint64_t generation, int flags) {
    struct amitia_bridge_fd *bridge = malloc(sizeof(*bridge));
    if (!bridge) return _ENOMEM;
    struct fd *fd = adhoc_fd_create(&bridge_operations);
    if (!fd) { free(bridge); return _ENOMEM; }
    fd->real_fd = dup(descriptor);
    if (fd->real_fd < 0) { fd_close(fd); free(bridge); return _EIO; }
    fcntl(fd->real_fd, F_SETFD, FD_CLOEXEC);
    *bridge = (struct amitia_bridge_fd){.generation = generation, .owner = owner};
    fd->fs_data = bridge;
    fd->flags = flags;
    fd->stat.mode = S_IFIFO | 0600;
    return f_install(fd, O_CLOEXEC_);
}

static void process_exit(struct task *task, int status) {
    for (struct amitia_process *item = processes; item; item = item->next) {
        if (item->pid == task->tgid) {
            item->exit_code = (status & 0x7f) ? 128 + (status & 0x7f) : (status >> 8) & 0xff;
            item->completed = true;
            break;
        }
    }
    if (previous_exit_hook) previous_exit_hook(task, status);
}

static char *string_blob(const char *const *items, size_t count) {
    if (count > 256 || (count && !items)) return NULL;
    size_t length = 1;
    for (size_t index = 0; index < count; index++) {
        if (!items[index]) return NULL;
        size_t size = strnlen(items[index], AMITIA_ARGUMENT_LIMIT);
        if (size == AMITIA_ARGUMENT_LIMIT || length + size + 1 > AMITIA_ARGUMENT_LIMIT) return NULL;
        length += size + 1;
    }
    char *blob = calloc(length, 1);
    if (!blob) return NULL;
    char *cursor = blob;
    for (size_t index = 0; index < count; index++) {
        size_t size = strlen(items[index]) + 1;
        memcpy(cursor, items[index], size);
        cursor += size;
    }
    return blob;
}

static bool valid_environment(const char *const *environment, size_t count) {
    if (count > 256 || (count && !environment)) return false;
    for (size_t index = 0; index < count; index++) {
        if (!environment[index] || !strchr(environment[index], '=') || environment[index][0] == '=') return false;
        size_t key = strcspn(environment[index], "=");
        if (strspn(environment[index], "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_0123456789") < key) return false;
        for (size_t previous = 0; previous < index; previous++)
            if (strcspn(environment[previous], "=") == key && !strncmp(environment[previous], environment[index], key)) return false;
    }
    return true;
}

static char *environment_blob(const char *base, const char *const *extra, size_t count) {
    if (!valid_environment(extra, count)) return NULL;
    const char *items[256];
    size_t total = 0;
    for (const char *item = base; item && *item; item += strlen(item) + 1) {
        size_t key = strcspn(item, "=");
        bool replaced = false;
        for (size_t index = 0; index < count; index++)
            if (strcspn(extra[index], "=") == key && !strncmp(item, extra[index], key)) replaced = true;
        if (!replaced) {
            if (total == 256) return NULL;
            items[total++] = item;
        }
    }
    if (total + count > 256) return NULL;
    for (size_t index = 0; index < count; index++) items[total++] = extra[index];
    return string_blob(items, total);
}

static int change_directory(const char *path) {
    if (!path || !*path) return 0;
    if (*path != '/') return _EINVAL;
    struct fd *directory = generic_open(path, O_RDONLY_ | O_DIRECTORY_, 0);
    if (IS_ERR(directory)) return PTR_ERR(directory);
    fs_chdir(current->fs, directory);
    return 0;
}

static void discard_unstarted_task(struct task *task) {
    mm_release(task->mm);
    fdtable_release(task->files);
    fs_info_release(task->fs);
    sighand_release(task->sighand);
    lock(&pids_lock);
    list_remove(&task->group_links);
    task_leave_session(task);
    list_remove(&task->group->pgroup);
    cond_destroy(&task->group->child_exit);
    cond_destroy(&task->group->stopped_cond);
    free(task->group);
    task_destroy(task);
    unlock(&pids_lock);
}

static int attach_stdio(int input, int output, int error) {
    int originals[3] = {input, output, error};
    for (int index = 0; index < 3; index++) {
        int copy = dup(originals[index]);
        if (copy < 0) return -1;
        struct fd *fd = adhoc_fd_create(&realfs_fdops);
        if (!fd) { close(copy); return -1; }
        fd->real_fd = copy;
        fd->dir = NULL;
        fd->flags = index ? O_WRONLY_ : O_RDONLY_;
        fd->stat.mode = S_IFIFO | 0600;
        current->files->files[index] = fd;
    }
    return 0;
}

static int prepare_devices(void) {
    const struct { const char *path; int major; int minor; } devices[] = {
        {"/dev/null", MEM_MAJOR, DEV_NULL_MINOR},
        {"/dev/zero", MEM_MAJOR, DEV_ZERO_MINOR},
        {"/dev/full", MEM_MAJOR, DEV_FULL_MINOR},
        {"/dev/random", MEM_MAJOR, DEV_RANDOM_MINOR},
        {"/dev/urandom", MEM_MAJOR, DEV_URANDOM_MINOR},
        {"/dev/tty", TTY_ALTERNATE_MAJOR, DEV_TTY_MINOR},
        {"/dev/ptmx", TTY_ALTERNATE_MAJOR, DEV_PTMX_MINOR}
    };
    for (size_t index = 0; index < sizeof(devices) / sizeof(devices[0]); index++) {
        int error = generic_mknodat(current->fs->pwd, devices[index].path, S_IFCHR | 0666, dev_make(devices[index].major, devices[index].minor));
        if (error < 0 && error != _EEXIST) return error;
    }
    return 0;
}

static int prepare_business_directories(void) {
    const struct { const char *path; int uid; int mode; } directories[] = {
        {"/var", 0, 0755}, {"/var/lib", 0, 0755},
        {"/var/lib/amitia", 0, 0700},
        {"/var/lib/amitia/config", 0, 0700}, {"/var/lib/amitia/data", 0, 0700},
        {"/var/lib/amitia/cache", 0, 0700}, {"/var/lib/amitia/log", 0, 0700},
        {"/var/lib/amitia/run", 0, 0700}, {"/var/lib/amitia/temp", 0, 0700},
        {"/var/lib/amitia/workspace", 0, 0700}, {"/var/lib/amitia/home", 0, 0700},
        {"/home", 0, 0755}, {"/home/amitia", 1000, 0750},
        {"/home/amitia/workspace", 1000, 0750}
    };
    for (size_t index = 0; index < sizeof(directories) / sizeof(directories[0]); index++) {
        const char *path = directories[index].path;
        int error = generic_mkdirat(current->fs->pwd, path, directories[index].mode);
        if (error < 0 && error != _EEXIST) return error;
        struct statbuf state;
        error = generic_statat(current->fs->pwd, path, &state, false);
        if (error < 0) return error;
        if (!S_ISDIR(state.mode)) return _EACCES;
        error = generic_setattrat(current->fs->pwd, path, make_attr(uid, directories[index].uid), false);
        if (error >= 0) error = generic_setattrat(current->fs->pwd, path, make_attr(gid, directories[index].uid), false);
        if (error >= 0) error = generic_setattrat(current->fs->pwd, path, make_attr(mode, S_IFDIR | directories[index].mode), false);
        if (error < 0) return error;
    }
    return 0;
}

static bool empty_directory_tree(const char *path) {
    DIR *directory = opendir(path);
    if (!directory) return errno == ENOENT;
    bool empty = true;
    struct dirent *entry;
    while (empty && (entry = readdir(directory))) {
        if (!strcmp(entry->d_name, ".") || !strcmp(entry->d_name, "..")) continue;
        char child[MAX_PATH + 1];
        struct stat state;
        if (snprintf(child, sizeof(child), "%s/%s", path, entry->d_name) >= (int)sizeof(child) || lstat(child, &state) || !S_ISDIR(state.st_mode) || !empty_directory_tree(child)) empty = false;
    }
    closedir(directory);
    return empty;
}

static int prepare_business_volume(const char *volume) {
    char data[MAX_PATH + 1], database[MAX_PATH + 1];
    if (snprintf(data, sizeof(data), "%s/data", volume) >= (int)sizeof(data) || snprintf(database, sizeof(database), "%s/meta.db", volume) >= (int)sizeof(database)) return _EINVAL;
    struct stat state;
    bool existing = lstat(database, &state) == 0;
    if (existing) {
        if (!S_ISREG(state.st_mode) || lstat(data, &state) || !S_ISDIR(state.st_mode)) return _EINVAL;
        return 0;
    }
    if (errno != ENOENT || !empty_directory_tree(volume)) return _EINVAL;
    if (mkdir(data, 0700) && errno != EEXIST) return _EIO;
    int descriptor = open(database, O_CREAT | O_EXCL | O_RDWR, 0600);
    if (descriptor < 0) return _EIO;
    close(descriptor);
    sqlite3 *db = NULL;
    sqlite3_stmt *statement = NULL;
    int error = sqlite3_open_v2(database, &db, SQLITE_OPEN_READWRITE, NULL);
    if (error == SQLITE_OK) error = sqlite3_exec(db,
        "BEGIN IMMEDIATE;"
        "CREATE TABLE meta (id INTEGER UNIQUE DEFAULT 0, db_inode INTEGER);"
        "INSERT INTO meta (db_inode) VALUES (0);"
        "CREATE TABLE stats (inode INTEGER PRIMARY KEY, stat BLOB);"
        "CREATE TABLE paths (path BLOB PRIMARY KEY, inode INTEGER REFERENCES stats(inode));"
        "CREATE INDEX inode_to_path ON paths (inode,path);"
        "PRAGMA user_version=3;", NULL, NULL, NULL);
    struct ish_stat root = {.mode = S_IFDIR | 0700, .uid = 0, .gid = 0};
    if (error == SQLITE_OK) error = sqlite3_prepare_v2(db, "INSERT INTO stats (inode,stat) VALUES (1,?)", -1, &statement, NULL);
    if (error == SQLITE_OK) error = sqlite3_bind_blob(statement, 1, &root, sizeof(root), SQLITE_TRANSIENT);
    if (error == SQLITE_OK) error = sqlite3_step(statement) == SQLITE_DONE ? SQLITE_OK : SQLITE_ERROR;
    if (statement) sqlite3_finalize(statement);
    if (error == SQLITE_OK) error = sqlite3_exec(db, "INSERT INTO paths (path,inode) VALUES (X'',1);COMMIT;", NULL, NULL, NULL);
    if (db) sqlite3_close(db);
    if (error != SQLITE_OK) { unlink(database); rmdir(data); return _EIO; }
    return 0;
}

int amitia_ish_start(const char *rootfs_path, const char *workdir, const char **env, size_t env_count) {
    if (!rootfs_path || !*rootfs_path || !valid_environment(env, env_count)) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    char candidate[MAX_PATH + 1], root[MAX_PATH + 1];
    struct stat state;
    if (snprintf(candidate, sizeof(candidate), "%s/data", rootfs_path) >= (int)sizeof(candidate)) return AMITIA_ISH_ERR_ROOTFS_NOT_READY;
    const char *source = stat(candidate, &state) == 0 && S_ISDIR(state.st_mode) ? candidate : rootfs_path;
    if (!realpath(source, root)) return AMITIA_ISH_ERR_ROOTFS_NOT_READY;
    const char *guest_env[256];
    size_t guest_count = 0;
    const char *business_host = NULL;
    const char business_key[] = "AMITIA_IOS_PERSISTENT_DATA_HOST=";
    for (size_t index = 0; index < env_count; index++) {
        if (!strncmp(env[index], business_key, sizeof(business_key) - 1)) business_host = env[index] + sizeof(business_key) - 1;
        else guest_env[guest_count++] = env[index];
    }
    char business_volume[MAX_PATH + 1];
    if (business_host) {
        struct stat state;
        if (*business_host != '/' || lstat(business_host, &state) || !S_ISDIR(state.st_mode) || !realpath(business_host, business_volume)) return AMITIA_ISH_ERR_ROOTFS_NOT_READY;
    }
    char *defaults = string_blob(default_environment, sizeof(default_environment) / sizeof(default_environment[0]));
    if (!defaults) return AMITIA_ISH_ERR_INTERNAL;
    char *environment = environment_blob(defaults, guest_env, guest_count);
    free(defaults);
    if (!environment) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    pthread_mutex_lock(&runtime_lock);
    if (mounted_root && strcmp(mounted_root, root)) {
        free(environment);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_ROOTFS_FORMAT_UNSUPPORTED;
    }
    if (mounted_business_volume && (!business_host || strcmp(mounted_business_volume, business_volume))) {
        free(environment);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_ROOTFS_FORMAT_UNSUPPORTED;
    }
    if (init_task && (!kernel_ready || atomic_load(&runtime_state) == AMITIA_ISH_ERROR)) {
        free(environment);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_INTERNAL;
    }
    struct task *saved = current;
    int error = 0;
    if (!init_task) {
        atomic_store(&runtime_state, AMITIA_ISH_STARTING);
        error = mount_root(&fakefs, root);
        if (error >= 0) error = become_first_process();
        if (error >= 0) {
            init_task = current;
            mounted_root = strdup(root);
            error = prepare_devices();
            if (error >= 0) error = do_mount(&procfs, "proc", "/proc", "", 0);
            if (error >= 0) error = do_mount(&devptsfs, "devpts", "/dev/pts", "", 0);
            lock(&pids_lock);
            previous_exit_hook = exit_hook;
            exit_hook = process_exit;
            unlock(&pids_lock);
            bridge_operations = realfs_fdops;
            bridge_operations.read = bridge_read;
            bridge_operations.write = bridge_write;
            bridge_operations.poll = bridge_poll;
            bridge_operations.close = bridge_close;
            kernel_ready = error >= 0;
        }
    }
    current = init_task;
    if (error >= 0 && init_task && business_host && !mounted_business_volume) {
        char old_data[MAX_PATH + 1], data[MAX_PATH + 1];
        if (snprintf(old_data, sizeof(old_data), "%s/var/lib/amitia", root) >= (int)sizeof(old_data) || !empty_directory_tree(old_data)) error = _EEXIST;
        if (error >= 0) error = prepare_business_volume(business_volume);
        if (error >= 0 && snprintf(data, sizeof(data), "%s/data", business_volume) >= (int)sizeof(data)) error = _EINVAL;
        if (error >= 0) error = do_mount(&fakefs, data, "/var/lib/amitia", "", 0);
        if (error >= 0) mounted_business_volume = strdup(business_volume);
        if (error >= 0 && !mounted_business_volume) error = _ENOMEM;
    }
    if (error >= 0 && init_task) error = prepare_business_directories();
    if (error >= 0 && init_task) error = change_directory(workdir);
    current = saved;
    if (error < 0 || !init_task || !mounted_root) {
        atomic_store(&runtime_state, AMITIA_ISH_ERROR);
        free(environment);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_ROOTFS_NOT_READY;
    }
    free(runtime_environment);
    runtime_environment = environment;
    atomic_store(&runtime_state, AMITIA_ISH_RUNNING);
    pthread_mutex_unlock(&runtime_lock);
    return AMITIA_ISH_OK;
}

int amitia_ish_spawn(const amitia_ish_command_t *command, amitia_ish_process_t *process) {
    if (!command || !process || command->argc < 1 || command->argc > 256 || !command->argv || !command->argv[0] || command->argv[0][0] != '/' || command->stdin_size > AMITIA_OUTPUT_LIMIT || (command->stdin_size && !command->stdin_data)) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    if (command->enable_host_bridge) {
        if (strcmp(command->argv[0], "/opt/amitia/core/AmitiaCore")) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
        const char *generation = NULL;
        const char generation_prefix[] = "AMITIA_IOS_HOST_BRIDGE_GENERATION=";
        if (command->env_count && !command->env) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
        for (size_t index = 0; index < command->env_count; index++) {
            const char *item = command->env[index];
            if (item && !strncmp(item, generation_prefix, sizeof(generation_prefix)-1)) generation = item + sizeof(generation_prefix)-1;
        }
        if (!generation || strlen(generation) < 16 || strlen(generation) > 128 || strpbrk(generation, "\r\n")) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    }
    *process = (amitia_ish_process_t){.stdin_fd = -1, .stdout_fd = -1, .stderr_fd = -1, .host_bridge_fd = -1};
    char *arguments = string_blob(command->argv, command->argc);
    if (!arguments) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    pthread_mutex_lock(&runtime_lock);
    if (atomic_load(&runtime_state) != AMITIA_ISH_RUNNING || !init_task) {
        free(arguments);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_NOT_INITIALIZED;
    }
    lock(&pids_lock);
    size_t count = 0;
    for (struct amitia_process *item = processes; item; item = item->next) count++;
    unlock(&pids_lock);
    if (count >= AMITIA_PROCESS_LIMIT || next_generation == UINT64_MAX) {
        free(arguments);
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_EXEC_BUSY;
    }
    char *environment = environment_blob(runtime_environment, command->env, command->env_count);
    if (command->enable_host_bridge && environment) {
        const char *bridge_env[] = {"AMITIA_IOS_HOST_BRIDGE_READ_FD=3", "AMITIA_IOS_HOST_BRIDGE_WRITE_FD=4", "AMITIA_IOS_HOST_BRIDGE_REQUIRED=true"};
        char *updated = environment_blob(environment, bridge_env, 3);
        free(environment);
        environment = updated;
    }
    if (!command->enable_host_bridge && environment) {
        const char *tool_env[] = {"HOME=/home/amitia"};
        char *updated = environment_blob(environment, tool_env, 1);
        free(environment);
        environment = updated;
    }
    struct amitia_process *record = calloc(1, sizeof(*record));
    int pipes[3][2] = {{-1,-1}, {-1,-1}, {-1,-1}};
    int bridge[2] = {-1,-1};
    struct task *saved = current;
    struct task *child = NULL;
    int error = AMITIA_ISH_ERR_INTERNAL;
    if (!environment || !record || pipe(pipes[0]) || pipe(pipes[1]) || pipe(pipes[2])) goto failed;
    if (command->enable_host_bridge && socketpair(AF_UNIX, SOCK_STREAM, 0, bridge)) goto failed;
    if (command->enable_host_bridge) { fcntl(bridge[0], F_SETFD, FD_CLOEXEC); fcntl(bridge[1], F_SETFD, FD_CLOEXEC); }
    for (int index = 0; index < 3; index++)
        for (int end = 0; end < 2; end++) fcntl(pipes[index][end], F_SETFD, FD_CLOEXEC);
    current = init_task;
    if (become_new_init_child() < 0) goto failed;
    child = current;
    if (attach_stdio(pipes[0][0], pipes[1][1], pipes[2][1]) < 0) goto failed;
    if (!command->enable_host_bridge) child->uid = child->euid = child->suid = child->gid = child->egid = child->sgid = 1000;
    const char *workdir = command->workdir ? command->workdir : command->enable_host_bridge ? NULL : "/home/amitia";
    if (change_directory(workdir) < 0) { error = AMITIA_ISH_ERR_INVALID_ARGUMENT; goto failed; }
    if (do_execve(command->argv[0], command->argc, arguments, environment) < 0) { error = AMITIA_ISH_ERR_EXEC_FAILED; goto failed; }
    record->generation = next_generation++;
    record->pid = child->pid;
    record->exit_code = -1;
    if (command->enable_host_bridge && (install_bridge_fd(bridge[1], child->tgid, record->generation, O_RDONLY_) != 3 || install_bridge_fd(bridge[1], child->tgid, record->generation, O_WRONLY_) != 4)) goto failed;
    lock(&pids_lock);
    record->next = processes;
    processes = record;
    unlock(&pids_lock);
    if (task_start(child) < 0) {
        lock(&pids_lock);
        processes = record->next;
        unlock(&pids_lock);
        error = AMITIA_ISH_ERR_EXEC_FAILED;
        goto failed;
    }
    current = saved;
    close(pipes[0][0]); close(pipes[1][1]); close(pipes[2][1]);
    process->generation = record->generation;
    process->stdin_fd = pipes[0][1];
    process->stdout_fd = pipes[1][0];
    process->stderr_fd = pipes[2][0];
    process->host_bridge_fd = bridge[0];
    if (bridge[1] >= 0) close(bridge[1]);
    free(arguments); free(environment);
    pthread_mutex_unlock(&runtime_lock);
    return AMITIA_ISH_OK;
failed:
    if (child) discard_unstarted_task(child);
    current = saved;
    for (int index = 0; index < 3; index++)
        for (int end = 0; end < 2; end++) if (pipes[index][end] >= 0) close(pipes[index][end]);
    if (bridge[0] >= 0) close(bridge[0]);
    if (bridge[1] >= 0) close(bridge[1]);
    free(record); free(arguments); free(environment);
    pthread_mutex_unlock(&runtime_lock);
    return error;
}

int amitia_ish_poll(uint64_t generation, bool *running, int *exit_code) {
    if (!running || !exit_code || !generation) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    lock(&pids_lock);
    struct amitia_process *item = find_process(generation);
    if (!item) { unlock(&pids_lock); return AMITIA_ISH_ERR_INVALID_ARGUMENT; }
    *running = !item->completed;
    *exit_code = item->completed ? item->exit_code : -1;
    unlock(&pids_lock);
    return AMITIA_ISH_OK;
}

int amitia_ish_reap(uint64_t generation, int *exit_code) {
    if (!exit_code || !generation) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    pthread_mutex_lock(&runtime_lock);
    lock(&pids_lock);
    struct amitia_process *item = find_process(generation);
    if (!item || !item->completed) {
        unlock(&pids_lock);
        pthread_mutex_unlock(&runtime_lock);
        return item ? AMITIA_ISH_ERR_EXEC_BUSY : AMITIA_ISH_ERR_INVALID_ARGUMENT;
    }
    pid_t_ pid = item->pid;
    int code = item->exit_code;
    unlock(&pids_lock);
    struct task *saved = current;
    current = init_task;
    int_t reaped = sys_wait4(pid, 0, AMITIA_WNOHANG, 0);
    current = saved;
    if (reaped != pid) {
        pthread_mutex_unlock(&runtime_lock);
        return AMITIA_ISH_ERR_INTERNAL;
    }
    lock(&pids_lock);
    struct amitia_process **link = &processes;
    while (*link && (*link)->generation != generation) link = &(*link)->next;
    if (*link) { item = *link; *link = item->next; free(item); }
    unlock(&pids_lock);
    *exit_code = code;
    pthread_mutex_unlock(&runtime_lock);
    return AMITIA_ISH_OK;
}

static void signal_process(uint64_t generation, int signal) {
    lock(&pids_lock);
    struct amitia_process *item = find_process(generation);
    if (item) {
        item->cancelled = true;
        struct task *leader = pid_get_task_zombie(item->pid);
        if (leader) {
            pid_t_ group = leader->group->pgid;
            for (int pid = 2; pid < MAX_PID; pid++) {
                struct task *task = pid_get_task(pid);
                if (task && task->group->pgid == group) send_signal(task, signal, SIGINFO_NIL);
            }
        }
    }
    unlock(&pids_lock);
}

int amitia_ish_cancel(uint64_t generation) {
    if (!generation) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    signal_process(generation, SIGTERM_);
    usleep(100000);
    signal_process(generation, SIGKILL_);
    return AMITIA_ISH_OK;
}

static int append_output(char **buffer, size_t *size, const char *data, size_t count) {
    if (count > AMITIA_OUTPUT_LIMIT - *size) return AMITIA_ISH_ERR_INTERNAL;
    char *expanded = realloc(*buffer, *size + count + 1);
    if (!expanded) return AMITIA_ISH_ERR_INTERNAL;
    memcpy(expanded + *size, data, count);
    *size += count;
    expanded[*size] = 0;
    *buffer = expanded;
    return AMITIA_ISH_OK;
}

int amitia_ish_execute(const amitia_ish_command_t *command, amitia_ish_result_t *result) {
    if (!result) return AMITIA_ISH_ERR_INVALID_ARGUMENT;
    memset(result, 0, sizeof(*result)); result->exit_code = -1;
    if (!command || command->enable_host_bridge) return result->error_code = AMITIA_ISH_ERR_INVALID_ARGUMENT;
    if (pthread_mutex_trylock(&short_execution_lock)) return result->error_code = AMITIA_ISH_ERR_EXEC_BUSY;
    amitia_ish_process_t process;
    int error = amitia_ish_spawn(command, &process);
    if (error != AMITIA_ISH_OK) {
        pthread_mutex_unlock(&short_execution_lock);
        return result->error_code = error;
    }
    result->generation = process.generation;
    fcntl(process.stdin_fd, F_SETFL, O_NONBLOCK);
    fcntl(process.stdout_fd, F_SETFL, O_NONBLOCK);
    fcntl(process.stderr_fd, F_SETFL, O_NONBLOCK);
    size_t written = 0;
    uint64_t deadline = monotonic_ms() + (command->timeout_ms ? command->timeout_ms : 30000);
    uint64_t exit_deadline = 0;
    bool output_open = true, error_open = true, running = true;
    char *output = NULL, *errors = NULL;
    while (running || output_open || error_open) {
        if (!exit_deadline && monotonic_ms() >= deadline) {
            error = AMITIA_ISH_ERR_EXEC_TIMEOUT;
            amitia_ish_cancel(process.generation);
            exit_deadline = monotonic_ms() + 5000;
        }
        if (exit_deadline && monotonic_ms() >= exit_deadline) { result->fatal = running; break; }
        if (process.stdin_fd >= 0 && written == command->stdin_size) { close(process.stdin_fd); process.stdin_fd = -1; }
        struct pollfd fds[3] = {
            {.fd = output_open ? process.stdout_fd : -1, .events = POLLIN},
            {.fd = error_open ? process.stderr_fd : -1, .events = POLLIN},
            {.fd = process.stdin_fd, .events = POLLOUT}
        };
        int ready = poll(fds, 3, 50);
        if (ready < 0 && errno != EINTR && !exit_deadline) { error = AMITIA_ISH_ERR_INTERNAL; amitia_ish_cancel(process.generation); exit_deadline = monotonic_ms() + 5000; }
        for (int index = 0; index < 2; index++) {
            if (!(fds[index].revents & (POLLIN | POLLHUP | POLLERR))) continue;
            char bytes[8192];
            ssize_t count = read(fds[index].fd, bytes, sizeof(bytes));
            if (!count || (count < 0 && errno != EAGAIN && errno != EINTR)) {
                if (index) error_open = false; else output_open = false;
            } else if (count > 0 && append_output(index ? &errors : &output, index ? &result->stderr_size : &result->stdout_size, bytes, count) != AMITIA_ISH_OK) {
                error = AMITIA_ISH_ERR_INTERNAL;
                amitia_ish_cancel(process.generation);
                if (!exit_deadline) exit_deadline = monotonic_ms() + 5000;
            }
        }
        if (process.stdin_fd >= 0 && fds[2].revents) {
            ssize_t count = write(process.stdin_fd, (const char *)command->stdin_data + written, command->stdin_size - written);
            if (count > 0) written += count;
            else if (count < 0 && errno != EAGAIN && errno != EINTR) { close(process.stdin_fd); process.stdin_fd = -1; }
        }
        if (amitia_ish_poll(process.generation, &running, &result->exit_code) != AMITIA_ISH_OK) { error = AMITIA_ISH_ERR_INTERNAL; break; }
        lock(&pids_lock);
        struct amitia_process *record = find_process(process.generation);
        bool cancelled = record && record->cancelled;
        unlock(&pids_lock);
        if (cancelled && error == AMITIA_ISH_OK) { error = AMITIA_ISH_ERR_EXEC_CANCELLED; exit_deadline = monotonic_ms() + 5000; }
        if (!running && !exit_deadline) exit_deadline = monotonic_ms() + 1000;
    }
    if (process.stdin_fd >= 0) close(process.stdin_fd);
    close(process.stdout_fd); close(process.stderr_fd);
    if (!running && amitia_ish_reap(process.generation, &result->exit_code) != AMITIA_ISH_OK) error = AMITIA_ISH_ERR_INTERNAL;
    result->stdout_data = output; result->stderr_data = errors; result->error_code = error;
    if (error != AMITIA_ISH_OK) result->error_message = strdup(error == AMITIA_ISH_ERR_EXEC_TIMEOUT ? "guest execution timed out" : error == AMITIA_ISH_ERR_EXEC_CANCELLED ? "guest execution cancelled" : "guest execution failed or exceeded output budget");
    if (result->fatal) atomic_store(&runtime_state, AMITIA_ISH_ERROR);
    pthread_mutex_unlock(&short_execution_lock);
    return error;
}

void amitia_ish_stop(void) {
    pthread_mutex_lock(&runtime_lock);
    atomic_store(&runtime_state, AMITIA_ISH_UNAVAILABLE);
    lock(&pids_lock);
    for (struct amitia_process *item = processes; item; item = item->next) item->cancelled = true;
    for (int pid = 2; pid < MAX_PID; pid++) {
        struct task *task = pid_get_task(pid);
        if (task) send_signal(task, SIGKILL_, SIGINFO_NIL);
    }
    unlock(&pids_lock);
    uint64_t deadline = monotonic_ms() + 5000;
    bool live = true;
    while (live && monotonic_ms() < deadline) {
        live = false;
        lock(&pids_lock);
        for (int pid = 2; pid < MAX_PID; pid++) if (pid_get_task(pid)) { live = true; break; }
        unlock(&pids_lock);
        if (live) usleep(10000);
    }
    if (live) atomic_store(&runtime_state, AMITIA_ISH_ERROR);
    pthread_mutex_unlock(&runtime_lock);
}

amitia_ish_state_t amitia_ish_state(void) { return atomic_load(&runtime_state); }

void amitia_ish_result_free(amitia_ish_result_t *result) {
    if (!result) return;
    free((void *)result->stdout_data); free((void *)result->stderr_data); free(result->error_message);
    memset(result, 0, sizeof(*result)); result->exit_code = -1;
}
