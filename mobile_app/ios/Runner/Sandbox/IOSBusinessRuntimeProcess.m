#import "IOSBusinessRuntimeProcess.h"
#include "amitia_ish_embed.h"
#include <unistd.h>

@implementation IOSBusinessRuntimeProcess {
    amitia_ish_process_t _process;
    BOOL _started;
}

- (instancetype)init {
    self = [super init];
    if (self) {
        _process.stdin_fd = -1;
        _process.stdout_fd = -1;
        _process.stderr_fd = -1;
        _process.host_bridge_fd = -1;
    }
    return self;
}

- (uint64_t)generation { return _process.generation; }
- (int)stdoutDescriptor { return _process.stdout_fd; }
- (int)stderrDescriptor { return _process.stderr_fd; }
- (int)hostBridgeDescriptor { return _process.host_bridge_fd; }

- (BOOL)spawn:(NSArray<NSString *> *)arguments environment:(NSDictionary<NSString *, NSString *> *)environment error:(NSError **)error {
    if (_started || arguments.count == 0) return [self fail:AMITIA_ISH_ERR_INVALID_ARGUMENT error:error];
    const char **argv = calloc(arguments.count + 1, sizeof(char *));
    const char **env = calloc(environment.count + 1, sizeof(char *));
    if (!argv || !env) {
        free(argv);
        free(env);
        return [self fail:AMITIA_ISH_ERR_INTERNAL error:error];
    }
    for (NSUInteger i = 0; i < arguments.count; i++) argv[i] = strdup(arguments[i].UTF8String);
    NSUInteger index = 0;
    for (NSString *key in environment) env[index++] = strdup([[NSString stringWithFormat:@"%@=%@", key, environment[key]] UTF8String]);
    amitia_ish_command_t command = {0};
    command.argc = (int)arguments.count;
    command.argv = argv;
    command.env = env;
    command.env_count = environment.count;
    command.workdir = "/";
    command.enable_host_bridge = true;
    int status = amitia_ish_spawn(&command, &_process);
    for (NSUInteger i = 0; i < arguments.count; i++) free((void *)argv[i]);
    for (NSUInteger i = 0; i < environment.count; i++) free((void *)env[i]);
    free(argv);
    free(env);
    if (status != AMITIA_ISH_OK) return [self fail:status error:error];
    _started = YES;
    if (_process.stdin_fd >= 0) { close(_process.stdin_fd); _process.stdin_fd = -1; }
    return YES;
}

- (BOOL)isRunning:(NSError **)error {
    if (!_started) return NO;
    bool running = false;
    int exitCode = 0;
    int status = amitia_ish_poll(_process.generation, &running, &exitCode);
    if (status != AMITIA_ISH_OK) return [self fail:status error:error];
    return running;
}

- (BOOL)cancel:(NSError **)error {
    if (!_started) return YES;
    int status = amitia_ish_cancel(_process.generation);
    return status == AMITIA_ISH_OK || [self fail:status error:error];
}

- (BOOL)reap:(NSError **)error {
    if (!_started) return YES;
    int exitCode = 0;
    int status = amitia_ish_reap(_process.generation, &exitCode);
    if (status != AMITIA_ISH_OK) return [self fail:status error:error];
    _started = NO;
    return YES;
}

- (BOOL)fail:(int)code error:(NSError **)error {
    if (error) *error = [NSError errorWithDomain:@"IOSBusinessRuntime" code:code userInfo:@{NSLocalizedDescriptionKey: @"iOS Runtime 进程操作失败"}];
    return NO;
}
@end
