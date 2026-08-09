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

@implementation AmitiaISHRuntimeHealth
@end

@interface AmitiaISHRuntime ()
@property (nonatomic, readwrite) AmitiaISHState state;
@property (nonatomic, strong) dispatch_queue_t executionQueue;
@property (nonatomic) uint64_t generation;
@property (nonatomic, copy, nullable) NSString *currentRootfsPath;
@property (nonatomic, copy, nullable) NSString *activeExecutionID;
@property (nonatomic) BOOL fatal;
@property (nonatomic, copy, nullable) NSString *fatalCode;
@property (nonatomic, strong, nullable) dispatch_cleanup_t executionCleanup;
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
        _generation = 0;
        _fatal = NO;
    }
    return self;
}

- (uint64_t)currentGeneration {
    return self.generation;
}

- (AmitiaISHState)state {
    return (AmitiaISHState)amitia_ish_state();
}

- (void)setState:(AmitiaISHState)state {
    if (state == AmitiaISHStateUnavailable) {
        self.fatal = NO;
        self.fatalCode = nil;
        self.generation = 0;
    }
    _state = state;
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
        int currentState = amitia_ish_state();
        if (currentState == AMITIA_ISH_RUNNING) {
            if ([self.currentRootfsPath isEqualToString:rootfsPath]) {
                success = YES;
                return;
            }
            amitia_ish_stop();
            self.generation = 0;
            self.currentRootfsPath = nil;
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
        if (success) {
            self.generation++;
            self.currentRootfsPath = [rootfsPath copy];
            self.fatal = NO;
            self.fatalCode = nil;
            self.activeExecutionID = nil;
        }
    });

    if (!success && error) {
        NSString *desc = [self descriptionForNativeError:result];
        NSError *err = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                            code:[self runtimeErrorCodeForNativeError:result]
                                        userInfo:@{NSLocalizedDescriptionKey: desc}];
        if (result == AMITIA_ISH_ERR_INTERNAL) {
            self.fatal = YES;
            self.fatalCode = @"ISH_INTERNAL_RUNTIME_FAULT";
            err = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                      code:AmitiaISHRuntimeErrorCodeInternal
                                  userInfo:@{NSLocalizedDescriptionKey: desc}];
        }
        *error = err;
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
        int currentState = amitia_ish_state();
        if (currentState != AMITIA_ISH_RUNNING) {
            if (error) {
                *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                             code:AmitiaISHRuntimeErrorCodeNotInitialized
                                         userInfo:@{NSLocalizedDescriptionKey: @"iSH kernel not running"}];
            }
            return;
        }

        uint64_t execGen = self.generation;
        NSString *execID = [[NSUUID UUID] UUIDString];
        self.activeExecutionID = execID;

        amitia_ish_command_t cmd = {0};
        cmd.argc = (int)argv.count;
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

        if (self.activeExecutionID && [self.activeExecutionID isEqualToString:execID]) {
            self.activeExecutionID = nil;
        }
        free(cArgv);

        if (rc != AMITIA_ISH_OK) {
            result = [[AmitiaISHExecutionResult alloc] init];
            result.exitCode = -1;
            result.stdout = @"";
            result.stderr = @"";
            result.errorCode = (AmitiaISHNativeError)rc;
            if (nativeResult.error_message) {
                result.errorMessage = [NSString stringWithUTF8String:nativeResult.error_message];
            }
            result.executionID = execID;
            result.generation = execGen;
            if (nativeResult.fatal) {
                result.fatal = YES;
                self.fatal = YES;
                self.fatalCode = result.errorMessage ?: @"ISH_INTERNAL_RUNTIME_FAULT";
            }
            if (error) {
                *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                             code:[self runtimeErrorCodeForNativeError:rc]
                                         userInfo:@{NSLocalizedDescriptionKey: result.errorMessage ?: [self descriptionForNativeError:rc]}];
            }
            return;
        }

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
        result.executionID = execID;
        result.generation = execGen;

        amitia_ish_result_free(&nativeResult);
    });

    return result;
}

- (BOOL)cancelExecution:(NSString *)executionId error:(NSError *_Nullable *_Nullable)error {
    if (!executionId || executionId.length == 0) {
        return [self cancelActiveExecution:error];
    }
    BOOL stillActive = [self.activeExecutionID isEqualToString:executionId];
    if (!stillActive) {
        if (error) {
            *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                         code:AmitiaISHRuntimeErrorCodeExecFailed
                                     userInfo:@{NSLocalizedDescriptionKey: @"execution ID not active"}];
        }
        return NO;
    }
    return [self cancelActiveExecution:error];
}

- (BOOL)cancelActiveExecution:(NSError *_Nullable *_Nullable)error {
    __block int rc = AMITIA_ISH_OK;
    dispatch_sync(self.executionQueue, ^{
        if (self.activeExecutionID) {
            int cancelRc = amitia_ish_cancel((uint64_t)self.activeExecutionID.longLongValue);
            if (cancelRc != AMITIA_ISH_OK) {
                rc = cancelRc;
            }
        }
        self.activeExecutionID = nil;
    });
    if (rc != AMITIA_ISH_OK && error) {
        *error = [NSError errorWithDomain:kAmitiaISHRuntimeErrorDomain
                                     code:AmitiaISHRuntimeErrorCodeExecFailed
                                 userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"cancel failed (%d)", rc]}];
        return NO;
    }
    return YES;
}

- (nullable NSString *)activeExecutionID {
    return _activeExecutionID;
}

- (void)stop {
    dispatch_sync(self.executionQueue, ^{
        if (self.activeExecutionID) {
            amitia_ish_cancel((uint64_t)self.activeExecutionID.longLongValue);
            self.activeExecutionID = nil;
        }
        amitia_ish_stop();
        self.generation = 0;
        self.currentRootfsPath = nil;
        self.fatal = NO;
        self.fatalCode = nil;
    });
    self.state = AmitiaISHStateUnavailable;
}

- (nullable AmitiaISHRuntimeHealth)health {
    __block AmitiaISHRuntimeHealth *h;
    dispatch_sync(self.executionQueue, ^{
        h = [[AmitiaISHRuntimeHealth alloc] init];
        h.kernelRunning = (amitia_ish_state() == AMITIA_ISH_RUNNING);
        h.initTaskValid = h.kernelRunning && (self.generation > 0);
        h.fatal = self.fatal;
        h.fatalCode = self.fatalCode;
        h.kernelRootfsPath = self.currentRootfsPath;
    });
    return h;
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
