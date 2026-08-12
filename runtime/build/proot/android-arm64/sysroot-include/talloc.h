#ifndef STUB_TALLOC_H
#define STUB_TALLOC_H

#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <stdarg.h>

typedef void TALLOC_CTX;

/* Auto-free context stub */
static inline TALLOC_CTX *talloc_autofree_context(void) {
    static TALLOC_CTX *autofree = NULL;
    if (!autofree) autofree = calloc(1, sizeof(void *));
    return autofree;
}

/* Internal: allocate zeroed memory */
static inline void *_talloc_zero(const void *ctx, size_t size, const char *name) {
    (void)ctx; (void)name;
    return calloc(1, size);
}

/* Internal: allocate array memory */
static inline void *_talloc_array_size(const void *ctx, size_t count, size_t el_size, const char *name) {
    (void)ctx; (void)name;
    return calloc(count > 0 ? count : 1, el_size);
}

static inline char *talloc_strdup(const void *ctx, const char *str) {
    if (!str) return NULL;
    size_t len = strlen(str) + 1;
    char *dup = (char *)malloc(len);
    if (dup) memcpy(dup, str, len);
    return dup;
}

static inline char *talloc_strndup(const void *ctx, const char *str, size_t n) {
    if (!str) return NULL;
    size_t len = strnlen(str, n);
    char *dup = (char *)malloc(len + 1);
    if (dup) {
        memcpy(dup, str, len);
        dup[len] = '\0';
    }
    return dup;
}

static inline char *talloc_asprintf(const void *ctx, const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    int len = vsnprintf(NULL, 0, fmt, ap);
    va_end(ap);
    if (len < 0) return NULL;
    char *buf = (char *)malloc(len + 1);
    if (buf) {
        va_start(ap, fmt);
        vsnprintf(buf, len + 1, fmt, ap);
        va_end(ap);
    }
    return buf;
}

/* talloc_strdup_append_buffer: append str to existing buffer */
static inline char *talloc_strdup_append_buffer(char *buffer, const char *str) {
    if (!str) return buffer;
    size_t blen = buffer ? strlen(buffer) : 0;
    size_t slen = strlen(str);
    char *newbuf = (char *)realloc(buffer, blen + slen + 1);
    if (newbuf) {
        memcpy(newbuf + blen, str, slen + 1);
    }
    return newbuf;
}

static inline void *talloc_realloc_size(const void *ctx, void *ptr, size_t size) {
    (void)ctx;
    return realloc(ptr, size);
}

static inline void talloc_free(void *ptr) {
    free(ptr);
}

static inline void *talloc_memdup(const void *ctx, const void *ptr, size_t size) {
    (void)ctx;
    if (!ptr) return NULL;
    void *dup = malloc(size);
    if (dup) memcpy(dup, ptr, size);
    return dup;
}

static inline void *talloc_new(const void *ctx) {
    (void)ctx;
    return calloc(1, sizeof(void *));
}

static inline void talloc_set_name_const(const void *ptr, const char *name) {
    (void)ptr; (void)name;
}

static inline const char *talloc_get_name(const void *ptr) {
    (void)ptr;
    return "talloc_stub";
}

static inline void *talloc_check_name(const void *ptr, const char *name) {
    (void)name;
    return (void *)ptr;
}

static inline size_t talloc_get_size(const void *ptr) {
    (void)ptr;
    return 0;
}

static inline void talloc_set_destructor(const void *ptr, int (*destructor)(void *)) {
    (void)ptr; (void)destructor;
}

static inline void *_talloc_steal_loc(const void *new_ctx, const void *ptr) {
    (void)new_ctx;
    return (void *)ptr;
}

static inline const void *talloc_parent(const void *ptr) {
    (void)ptr;
    return NULL;
}

/* No-op report functions */
static inline void talloc_enable_leak_report(void) { }
static inline void talloc_enable_leak_report_full(void) { }

/* talloc_array_length: stub returns 0 */
static inline size_t talloc_array_length(const void *ptr) {
    (void)ptr;
    return 0;
}

/* talloc_reparent: stub returns ptr */
static inline void *talloc_reparent(const void *old_parent, const void *new_parent, const void *ptr) {
    (void)old_parent; (void)new_parent;
    return (void *)ptr;
}

/* talloc_reference_count: stub returns 0 */
static inline int talloc_reference_count(const void *ptr) {
    (void)ptr;
    return 0;
}

/* Report depth callbacks: no-ops */
static inline void talloc_report_depth_cb(const void *ptr, int depth, int max_depth,
                                          void (*callback)(const void *ptr, int depth, int max_depth,
                                                          int is_ref, void *private_data),
                                          void *private_data) {
    (void)ptr; (void)depth; (void)max_depth; (void)callback; (void)private_data;
}

static inline void talloc_report_depth_file(const void *ptr, int depth, int max_depth, FILE *f) {
    (void)ptr; (void)depth; (void)max_depth; (void)f;
}

static inline void talloc_report_full(const void *ptr, FILE *f) {
    (void)ptr; (void)f;
}

static inline void talloc_report(const void *ptr, FILE *f) {
    (void)ptr; (void)f;
}

/* Type-aware macros */
#define talloc(ctx, type) (type *)_talloc_zero(ctx, sizeof(type), #type)
#define talloc_zero(ctx, type) (type *)_talloc_zero(ctx, sizeof(type), #type)
#define talloc_array(ctx, type, count) (type *)_talloc_array_size(ctx, count, sizeof(type), #type "[]")
#define talloc_zero_array(ctx, type, count) (type *)_talloc_array_size(ctx, count, sizeof(type), #type "[]")
#define talloc_size(ctx, size) _talloc_zero(ctx, size, "size")
#define talloc_zero_size(ctx, size) _talloc_zero(ctx, size, "zero_size")
#define talloc_realloc(ctx, p, type, count) (type *)talloc_realloc_size(ctx, p, sizeof(type) * (count))
#define talloc_steal(ctx, ptr) (_talloc_steal_loc(ctx, ptr))
#define talloc_reference(ctx, ptr) (ptr)
#define talloc_unlink(ctx, ptr) 0
#define talloc_get_type(ctx, type) (type *)talloc_check_name(ctx, #type)
#define talloc_get_type_abort(ctx, type) (type *)talloc_check_name(ctx, #type)
#define TALLOC_FREE(ctx) do { if (ctx) { talloc_free(ctx); (ctx) = NULL; } } while(0)

#endif /* STUB_TALLOC_H */
