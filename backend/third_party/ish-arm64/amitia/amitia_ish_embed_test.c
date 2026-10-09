#include "amitia_ish_embed.c"
#include <assert.h>

__thread struct task *current;
lock_t pids_lock = LOCK_INITIALIZER;

int main(void) {
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
    struct amitia_bridge_fd bridge = {.generation = 7, .owner = 42};
    struct fd descriptor = {.fs_data = &bridge};
    assert(bridge_authorized(&descriptor));
    caller.tgid = 43;
    assert(!bridge_authorized(&descriptor));
    caller.tgid = 42;
    bridge.generation = 8;
    assert(!bridge_authorized(&descriptor));
    bridge.generation = 7;
    record.cancelled = true;
    assert(!bridge_authorized(&descriptor));
    record.cancelled = false;
    bool running = false;
    int status = 0;
    assert(amitia_ish_poll(7, &running, &status) == 0 && running && status == -1);
    lock(&pids_lock);
    process_exit(&caller, 9);
    unlock(&pids_lock);
    assert(amitia_ish_poll(7, &running, &status) == 0 && !running && status == 137);
    assert(!bridge_authorized(&descriptor));
    assert(amitia_ish_poll(8, &running, &status) == AMITIA_ISH_ERR_INVALID_ARGUMENT);
    current = NULL; processes = NULL;
    char *output = NULL;
    size_t size = AMITIA_OUTPUT_LIMIT;
    assert(append_output(&output, &size, "x", 1) == AMITIA_ISH_ERR_INTERNAL);
    puts("amitia iSH environment/generation/owner/cancel/exit/output checks passed");
    return 0;
}
