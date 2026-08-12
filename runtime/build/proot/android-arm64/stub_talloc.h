#ifndef STUB_TALLOC_H
#define STUB_TALLOC_H

#include <stdlib.h>
#include <string.h>
#include <stdio.h>

typedef void TALLOC_CTX;

static inline void *talloc_zero_size(const void *ctx, size_t size) {
    void *ptr = calloc(1, size);
    return ptr;
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

static inline void *talloc_array_size(const void *ctx, size_t count, size_t size) {
    void *ptr = calloc(count > 0 ? count : 1, size);
    return ptr;
}

static inline void *talloc_realloc_size(const void *ctx, void *ptr, size_t size) {
    return realloc(ptr, size);
}

static inline void talloc_free(void *ptr) {
    free(ptr);
}

static inline void *talloc_zero(const void *ctx, size_t size) {
    return talloc_zero_size(ctx, size);
}

static inline void *talloc_memdup(const void *ctx, const void *ptr, size_t size) {
    if (!ptr) return NULL;
    void *dup = malloc(size);
    if (dup) memcpy(dup, ptr, size);
    return dup;
}

static inline void *talloc_new(const void *ctx) {
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

#define talloc(ctx, type) (type *)talloc_zero(ctx, sizeof(type))
#define talloc_array(ctx, type, count) (type *)talloc_zero_size(ctx, (count) * sizeof(type))
#define talloc_size(ctx, size) talloc_zero_size(ctx, size)
#define talloc_realloc(ctx, p, type, count) (type *)talloc_realloc_size(ctx, p, (count) * sizeof(type))
#define talloc_steal(ctx, ptr) (_talloc_steal_loc(ctx, ptr))
#define talloc_reference(ctx, ptr) (ptr)
#define talloc_unlink(ctx, ptr) 0
#define TALLOC_FREE(ctx) do { if (ctx) { talloc_free(ctx); (ctx) = NULL; } } while(0)

#endif /* STUB_TALLOC_H */
