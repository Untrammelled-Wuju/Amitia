#include "amitia_ish_embed.c"
#include <assert.h>

__thread struct task *current;
lock_t pids_lock = LOCK_INITIALIZER;
static struct task *test_leader;

struct task *pid_get_task_zombie(dword_t pid) { return test_leader && test_leader->pid == pid ? test_leader : NULL; }

static void test_business_volume(void) {
    char volume[] = "/tmp/amitia-volume-XXXXXX";
    assert(mkdtemp(volume));
    assert(prepare_business_volume(volume) == 0);
    char database[MAX_PATH + 1], data[MAX_PATH + 1], memory[MAX_PATH + 1];
    snprintf(database, sizeof(database), "%s/meta.db", volume);
    snprintf(data, sizeof(data), "%s/data", volume);
    snprintf(memory, sizeof(memory), "%s/data/memory.json", volume);
    sqlite3 *db = NULL;
    sqlite3_stmt *statement = NULL;
    assert(sqlite3_open_v2(database, &db, SQLITE_OPEN_READONLY, NULL) == SQLITE_OK);
    assert(sqlite3_prepare_v2(db, "SELECT stat FROM stats JOIN paths USING(inode) WHERE path=X''", -1, &statement, NULL) == SQLITE_OK);
    assert(sqlite3_step(statement) == SQLITE_ROW);
    struct ish_stat root;
    assert(sqlite3_column_bytes(statement, 0) == sizeof(root));
    memcpy(&root, sqlite3_column_blob(statement, 0), sizeof(root));
    assert(root.uid == 0 && root.gid == 0 && root.mode == (S_IFDIR | 0700));
    sqlite3_finalize(statement); sqlite3_close(db);
    struct stat state;
    assert(!stat(database, &state) && (state.st_mode & 0777) == 0600);
    FILE *file = fopen(memory, "w");
    assert(file); fputs("existing owner memory", file); fclose(file);
    assert(!empty_directory_tree(data));
    assert(prepare_business_volume(volume) == 0);
    assert(!stat(memory, &state) && state.st_size == 21);
    assert(!unlink(database));
    assert(prepare_business_volume(volume) == _EINVAL);
    assert(!stat(memory, &state) && state.st_size == 21);
    assert(!unlink(memory) && !rmdir(data) && !rmdir(volume));
    char unexpected[] = "/tmp/amitia-volume-XXXXXX";
    assert(mkdtemp(unexpected));
    snprintf(database, sizeof(database), "%s/meta.db", unexpected);
    assert(!symlink("/etc/passwd", database));
    assert(prepare_business_volume(unexpected) == _EINVAL);
    assert(!unlink(database) && !rmdir(unexpected));
}

int main(void) {
    assert(!amitia_ish_root_mounted());
    mounted_root = "/test/root/data";
    atomic_store(&runtime_state, AMITIA_ISH_UNAVAILABLE);
    assert(amitia_ish_root_mounted());
    mounted_root = NULL;
    const char *initial[] = {"HOME=/root", "PATH=/bin", "MODE=old"};
    const char *updates[] = {"MODE=new", "NEW=yes"};
    char *base = string_blob(initial, 3);
    char *merged = environment_blob(base, updates, 2);
    assert(merged && !strcmp(merged, "HOME=/root"));
    size_t count = 0;
    bool old = false, changed = false;
    for (char *item = merged; *item; item += strlen(item) + 1) {
        count++;
        old |= !strcmp(item, "MODE=old");
        changed |= !strcmp(item, "MODE=new");
    }
    assert(count == 4 && !old && changed);
    const char *duplicates[] = {"MODE=a", "MODE=b"};
    assert(!environment_blob(base, duplicates, 2));
    assert(!string_blob(NULL, 1));
    free(base); free(merged);

    struct task caller = {.tgid = 42};
    current = &caller;
    struct amitia_process record = {.generation = 7, .pid = 42, .exit_code = -1};
    processes = &record;
    struct amitia_bridge_fd bridge = {.generation = 7, .owner = 42, .active = true, .references = 1};
    record.bridge = &bridge;
    struct fd descriptor = {.fs_data = &bridge};
    assert(bridge_authorized(&descriptor));
    lock(&pids_lock);
    assert(bridge_authorized(&descriptor));
    unlock(&pids_lock);
    caller.tgid = 43;
    assert(!bridge_authorized(&descriptor));
    caller.tgid = 42;
    bridge.generation = 0;
    assert(!bridge_authorized(&descriptor));
    bridge.generation = 7;
    record.cancelled = true;
    atomic_store(&bridge.active, false);
    assert(!bridge_authorized(&descriptor));
    record.cancelled = false;
    atomic_store(&bridge.active, true);
    bool running = false;
    int status = 0;
    assert(amitia_ish_poll(7, &running, &status) == 0 && running && status == -1);
    lock(&pids_lock);
    process_exit(&caller, 9);
    unlock(&pids_lock);
    assert(amitia_ish_poll(7, &running, &status) == 0 && !running && status == 137);
    assert(!bridge_authorized(&descriptor));
    assert(amitia_ish_poll(8, &running, &status) == AMITIA_ISH_ERR_INVALID_ARGUMENT);
    struct amitia_bridge_fd replacement = {.generation = 8, .owner = 42, .active = true, .references = 1};
    struct fd new_descriptor = {.fs_data = &replacement};
    assert(bridge_authorized(&new_descriptor) && !bridge_authorized(&descriptor));
    struct tgroup group = {.doing_group_exit = true, .group_exit_code = 3 << 8};
    struct task zombie = {.pid = 42, .zombie = true, .group = &group};
    test_leader = &zombie;
    record.completed = false;
    assert(amitia_ish_poll(7, &running, &status) == 0 && !running && status == 3);
    test_leader = NULL;
    current = NULL; processes = NULL;
    char *output = NULL;
    size_t size = AMITIA_OUTPUT_LIMIT;
    assert(append_output(&output, &size, "x", 1) == AMITIA_ISH_ERR_INTERNAL);
    test_business_volume();
    puts("amitia iSH environment/generation/owner/cancel/exit/output checks passed");
    return 0;
}
