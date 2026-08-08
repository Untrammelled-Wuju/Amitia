#import "AmitiaISHRuntime.h"
#import "amitia_ish_embed.h"

static const NSErrorDomain kAmitiaISHRuntimeErrorDomain = @"com.amitia.AmitiaISHRuntime";

typedef NS_ENUM(NSInteger, AmitiaISHRuntimeErrorCode) {
    AmitiaISHRuntimeErrorCodeUnavailable = 2000,
    AmitiaISHRuntimeErrorCodeNotInitialized,
    AmitiaISHRuntimeErrorCodeRootfsNotReady,
    AmitiaISHRuntimeErrorCodeRootfsUnsupported,
    AmitiaISHRuntimeErrorCodeInvalidArgument,
    AmitiaISHRuntimeErrorCodeExecFailed,
    AmitiaISHRuntimeErrorCodeExecTimeout,
    AmitiaISHRuntimeErrorCodeExecCancelled,
    AmitiaISHRuntimeErrorCodeExecBusy,
    AmitiaISHRuntimeErrorCodeInternal,
};

@implementation AmitiaISHExecutionResult
@end

@interface AmitiaISHRuntime ()
@property (nonatomic, readwrite) AmitiaISHState state;
@property (nonatomic, strong)dispatch_queue_t executionQueue;
@end

@implementation AmitiaISHRuntime

+ (instancetype)shared {
    static AmitiaISHRuntime *instance;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        instance = [[AmitiaISHRuntime alloc] init];
    });
    return instance;
}

- (instancetype)init {
    self = [super init];
    if (self) {
        _state = AmitiaISHStateUnavailable;
        _executionQueue = dispatch_queue_create("com.amitia.ish.runtime", DISPATCH_QUEUE_SERIAL);
    }
    return self;
}

- (AmitiaISHState)state {
    return (AmitiaISHState)amitia_ish_state();
}

- (BOOL)startWithRootfsPath:(NSString *)rootfsPath
                   workdir:(nullable NSString *)workdir
                environment:(nullable NSDictionary<NSString *, NSString *> *)env
                      error:(NSError *_Nullable *_Nullable)error {
    if (rootfsPath.length == 0) {
        if (error) {
            *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                         code:AmitiaISHRuntimeErrorCodeRootfsNotReady
                                     userInfo:@{NSLocalizedDescriptionKey: @"rootfs path is empty"}];
        }
        return NO;
    }

    __block int result = AMITIA_ISH_ERR_INVALID_ARGUMENT;
    __block BOOL success = NO;

    dispatch_sync(self.executionQueue, ^{
        if (amitia_ish_state() == AMITIA_ISH_RUNNING) {
            success = YES;
            return;
        }

        size_t envCount = env.count;
        const char **cEnv = NULL;
        if (envCount > 0) {
            cEnv = malloc(sizeof(const char *) * envCount);
            NSUInteger idx = 0;
            for (NSString *key in env) {
                NSString *val = env[key];
                NSString *pair = [NSString stringWithFormat:@"%@=%@", key, val];
                cEnv[idx++] = pair.UTF8String;
            }
        }

        const char *cWorkdir = workdir ? workdir.UTF8String : "/root";
        const char *cRootfs = rootfsPath.UTF8String;

        result = amitia_ish_start(cRootfs, cWorkdir, cEnv, envCount);
        if (cEnv) free(cEnv);

        success = (result == AMITIA_ISH_OK);
    });

    if (!success && error) {
        NSString *desc = [self descriptionForNativeError:result];
        *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                     code:[self runtimeErrorCodeForNativeError:result]
                                 userInfo:@{NSLocalizedDescriptionKey: desc}];
    }

    return success;
}

- (nullable AmitiaISHExecutionResult *)executeCommand:(NSArray<NSString *> *)argv
                                                 stdin:(nullable NSString *)stdin
                                              workdir:(nullable NSString *)workdir
                                              timeout:(NSInteger)timeout
                                                error:(NSError *_Nullable *_Nullable)error {
    if (argv.count == 0) {
        if (error) {
            *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                         code:AmitiaISHRuntimeErrorCodeInvalidArgument
                                     userInfo:@{NSLocalizedDescriptionKey: @"empty argv"}];
        }
        return nil;
    }

    __block AmitiaISHExecutionResult *result = nil;

    dispatch_sync(self.executionQueue, ^{
        if (amitia_ish_state() != AMITIA_ISH_RUNNING) {
            if (error) {
                *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                             code:AmitiaISHRuntimeErrorCodeNotInitialized
                                         userInfo:@{NSLocalizedDescriptionKey: @"iSH kernel not running"}];
            }
            return;
        }

        amitia_ish_command_t cmd = {0};
        cmd.argc = argv.size;
        const char **cArgv = malloc(sizeof(const char *) * argv.count);
        for (NSUInteger i = 0; i < argv.count; i++) {
            cArgv[i] = [argv[i] UTF8String];
        }
        cmd.argv = cArgv;

        if (stdin.length > 0) {
            NSData *stdinData = [stdin dataUsingEncoding:NSUTF8StringEncoding];
            cmd.stdin_data = stdinData.bytes;
            cmd.stdin_size = stdinData.length;
        }

        cmd.workdir = workdir ? workdir.UTF8String : NULL;
        cmd.timeout_ms = (timeout > 0) ? (uint32_t)(timeout * 1000) : 0;

        amitia_ish_result_t nativeResult = {0};
        int rc = amitia_ish_execute(&cmd, &nativeResult);
        free(cArgv);

        result = [[AmitiaISHExecutionResult alloc] init];
        result.exitCode = nativeResult.exit_code;
        if (nativeResult.stdout_size > 0 && nativeResult.stdout_data) {
            result.stdout = [[NSString alloc] initWithBytes:nativeResult.stdout_data
                                                    length:nativeResult.stdout_size
                                                  encoding:NSUTF8StringEncoding];
            if (!result.stdout) {
                result.stdout = [[NSString alloc] initWithBytes:nativeResult.stdout_data
                                                        length:nativeResult.stdout_size
                                                      encoding:NSISOLatin1StringEncoding];
            }
        } else {
            result.stdout = @"";
        }
        if (nativeResult.stderr_size > 0 && nativeResult.stderr_data) {
            result.stderr = [[NSString alloc] initWithBytes:nativeResult.stderr_data
                                                    length:nativeResult.stderr_size
                                                  encoding:NSUTF8StringEncoding];
            if (!result.stderr) {
                result.stderr = [[NSString alloc] initWithBytes:nativeResult.stderr_data
                                                        length:nativeResult.stderr_size
                                                      encoding:NSISOLatin1StringEncoding];
            }
        } else {
            result.stderr = @"";
        }
        result.errorCode = (AmitiaISHNativeError)nativeResult.error_code;
        if (nativeResult.error_message) {
            result.errorMessage = [NSString stringWithUTF8String:nativeResult.error_message];
        }

        amitia_ish_result_free(&nativeResult);

        if (error && rc != AMITIA_ISH_OK) {
            *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                         code:[self runtimeErrorCodeForNativeError:rc]
                                     userInfo:@{NSLocalizedDescriptionKey: result.errorMessage ?: [self descriptionForNativeError:rc]}];
        }
    });

    return result;
}

- (BOOL)cancelExecution:(NSString *)executionId error:(NSError *_Nullable *_Nullable)error {
    uint64_t execId = (uint64_t)executionId.longLongValue;
    __block int result = AMITIA_ISH_OK;

    dispatch_sync(self.executionQueue, ^{
        result = amitia_ish_cancel(execId);
    });

    return result == AMITIA_ISH_OK;
}

- (void)stop {
    dispatch_sync(self.executionQueue, ^{
        amitia_ish_stop();
    });
    self.state = AmitiaISHStateUnavailable;
}

- (NSString *)descriptionForNativeError:(int)err {
    switch (err) {
        case AMITIA_ISH_OK: return @"OK";
        case AMITIA_ISH_ERR_NOT_INITIALIZED: return @"iSH kernel not initialized";
        case AMITIA_ISH_ERR_ROOTFS_NOT_READY: return @"rootfs not ready";
        case AMITIA_ISH_ERR_ROOTFS_FORMAT_UNSUPPORTED: return @"rootfs format unsupported";
        case AMITIA_ISH_ERR_INVALID_ARGUMENT: return @"invalid argument";
        case AMITIA_ISH_ERR_EXEC_FAILED: return @"execution failed";
        case AMITIA_ISH_ERR_EXEC_TIMEOUT: return @"execution timed out";
        case AMITIA_ISH_ERR_EXEC_CANCELLED: return @"execution was cancelled";
        case AMITIA_ISH_ERR_EXEC_BUSY: return @"execution busy";
        case AMITIA_ISH_ERR_INTERNAL: return @"internal error";
        default: return [NSString stringWithFormat:@"unknown error (%d)", err];
    }
}

- (NSInteger)runtimeErrorCodeForNativeError:(int)err {
    switch (err) {
        case AMITIA_ISH_OK: return 0;
        case AMITIA_ISH_ERR_NOT_INITIALIZED: return AmitiaISHRuntimeErrorCodeNotInitialized;
        case AMITIA_ISH_ERR_ROOTFS_NOT_READY: return AmitiaISHRuntimeErrorCodeRootfsNotReady;
        case AMITIA_ISH_ERR_ROOTFS_FORMAT_UNSUPPORTED: return AmitiaISHRuntimeErrorCodeRootfsUnsupported;
        case AMITIA_ISH_ERR_INVALID_ARGUMENT: return AmitiaISHRuntimeErrorCodeInvalidArgument;
        case AMITIA_ISH_ERR_EXEC_FAILED: return AmitiaISHRuntimeErrorCodeExecFailed;
        case AMITIA_ISH_ERR_EXEC_TIMEOUT: return AmitiaISHRuntimeErrorCodeExecTimeout;
        case AMITIA_ISH_ERR_EXEC_CANCELLED: return AmitiaISHRuntimeErrorCodeExecCancelled;
        case AMITIA_ISH_ERR_EXEC_BUSY: return AmitiaISHRuntimeErrorCodeExecBusy;
        case AMITIA_ISH_ERR_INTERNAL:
        default: return AmitiaISHRuntimeErrorCodeInternal;
    }
}

@end
