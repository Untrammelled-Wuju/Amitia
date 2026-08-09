#import "IOSSandboxBridge.h"
#import "AmitiaISHRuntime.h"

NSString * const kIOSSandboxBridgeErrorDomain = @"com.amitia.IOSSandboxBridge";
NSString * const kRootfsActiveVersionDidChangeNotification = @"com.amitia.RootfsActiveVersionDidChange";

NSString * const kIOSSandboxLifecycleStateName[] = {
    [ISHSandboxLifecycleIdle] = "idle",
    [ISHSandboxLifecycleStarting] = "starting",
    [ISHSandboxLifecycleRunning] = "running",
    [ISHSandboxLifecycleQuiescing] = "quiescing",
    [ISHSandboxLifecycleQuiesced] = "quiesced",
    [ISHSandboxLifecycleStopping] = "stopping",
    [ISHSandboxLifecycleFailed] = "failed",
};

static const uint64_t kRecoveryMaxAttempts = 3;
static const NSTimeInterval kRecoveryWindowSeconds = 120.0;
static const NSTimeInterval kRecoveryDelays[] = {0.25, 1.0, 3.0};
static const NSTimeInterval kStableWindowSeconds = 300.0;
static const NSTimeInterval kGuestCancelGraceMillis = 1500.0;
static const NSTimeInterval kGuestKillGraceMillis = 500.0;
static const NSInteger kMaxBackgroundTaskSeconds = 30;

typedef NS_ENUM(NSInteger, LifecycleStartResult) {
    LifecycleStartResultStarted = 0,
    LifecycleStartResultAlreadyRunning,
    LifecycleStartResultStartInProgress,
};

typedef NS_ENUM(NSInteger, LifecycleStopResult) {
    LifecycleStopResultStopped = 0,
    LifecycleStopResultIdle,
    LifecycleStopResultStopInProgress,
};

@interface IOSSandboxBridge ()

@property (nonatomic, readwrite, strong) AmitiaISHRuntime *runtime;
@property (nonatomic, readwrite, strong, nullable) RootfsResolver *resolver;

@property (nonatomic) ISHSandboxLifecycleState lifecycleState;
@property (nonatomic) ISHAvailability availability;
@property (nonatomic, strong, nullable) ISHBridgeConfig *currentConfig;
@property (nonatomic) BOOL ishInitialized;
@property (nonatomic) BOOL rootfsInstalled;
@property (nonatomic) BOOL desiredRunning;
@property (nonatomic) BOOL restartRequired;
@property (nonatomic) BOOL resumeOnForeground;
@property (nonatomic) BOOL recoveryPending;
@property (nonatomic) uint64_t generation;
@property (nonatomic, copy, nullable) NSString *runningRootfsVersion;
@property (nonatomic, copy, nullable) NSString *runningRootfsDigest;
@property (nonatomic, copy, nullable) NSString *runningRootfsPath;
@property (nonatomic, copy, nullable) NSString *activeExecutionID;
@property (nonatomic, copy, nullable) NSString *lastErrorCode;
@property (nonatomic, strong, nullable) NSDate *lastTransitionAt;
@property (nonatomic, strong, nullable) NSDate *lastStableAt;

@property (nonatomic, strong) dispatch_queue_t lifecycleQueue;
@property (nonatomic, strong) dispatch_queue_t recoveryQueue;
@property (nonatomic, copy, nullable) NSString *pendingStopReason;
@property (nonatomic, copy, nullable) NSString *pendingRestartReason;
@property (nonatomic, strong, nullable) NSMutableArray<NSDate *> *recoveryAttempts;
@property (nonatomic, weak, nullable) id<NSObject> rootfsObserver;
@property (nonatomic) UIBackgroundTaskIdentifier backgroundTaskID;
@property (nonatomic, copy, nullable) void (^pendingCompletion)(BOOL, NSError *);

@property (nonatomic, copy, nullable) NSDate (^clock)(void);

@end

@implementation IOSSandboxBridge

+ (instancetype)shared {
    return [self sharedWithResolver:nil];
}

+ (instancetype)sharedWithResolver:(RootfsResolver *)resolver {
    static IOSSandboxBridge *instance;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        instance = [[IOSSandboxBridge alloc] initWithResolver:resolver];
    });
    return instance;
}

- (instancetype)initWithResolver:(RootfsResolver *)resolver {
    self = [super init];
    if (self) {
        _runtime = [AmitiaISHRuntime shared];
        _resolver = resolver ?: [[RootfsResolver alloc] init];
        _lifecycleState = ISHSandboxLifecycleIdle;
        _availability = ISHAvailabilityUnavailable;
        _ishInitialized = NO;
        _rootfsInstalled = NO;
        _desiredRunning = NO;
        _restartRequired = NO;
        _resumeOnForeground = NO;
        _recoveryPending = NO;
        _generation = 0;
        _lifecycleQueue = dispatch_queue_create("com.amitia.ios-sandbox.lifecycle", DISPATCH_QUEUE_SERIAL);
        _recoveryQueue = dispatch_queue_create("com.amitia.ios-sandbox.recovery", DISPATCH_QUEUE_SERIAL);
        _recoveryAttempts = [NSMutableArray array];
        _backgroundTaskID = UIBackgroundTaskInvalid;
        _clock = nil;
        [self observeRootfsChanges];
    }
    return self;
}

- (instancetype)init {
    return [self initWithResolver:nil];
}

- (void)dealloc {
    if (_backgroundTaskID != UIBackgroundTaskInvalid) {
        [[UIApplication sharedApplication] endBackgroundTask:_backgroundTaskID];
        _backgroundTaskID = UIBackgroundTaskInvalid;
    }
    if (_rootfsObserver) {
        [[NSNotificationCenter defaultCenter] removeObserver:_rootfsObserver];
    }
}

#pragma mark - Clock

- (NSDate *)now {
    if (self.clock) return self.clock();
    return [NSDate date];
}

#pragma mark - Rootfs Observation

- (void)observeRootfsChanges {
    __weak typeof(self) weakSelf = self;
    id<NSObject> observer = [[NSNotificationCenter defaultCenter]
        addObserverForName:kRootfsActiveVersionDidChangeNotification
                    object:nil
                     queue:[NSOperationQueue mainQueue]
                usingBlock:^(NSNotification *note) {
        __strong typeof(weakSelf) self = weakSelf;
        if (self != nil) {
            [self handleRootfsDidChange];
        }
    }];
    self.rootfsObserver = observer;
}

- (void)handleRootfsDidChange {
    dispatch_async(self.lifecycleQueue, ^{
        self.restartRequired = YES;
        self.lastErrorCode = @"ROOTFS_CHANGED";
    });
}

#pragma mark - State Transition

- (BOOL)isValidTransitionFrom:(ISHSandboxLifecycleState)from to:(ISHSandboxLifecycleState)to {
    switch (from) {
        case ISHSandboxLifecycleIdle:
            return to == ISHSandboxLifecycleStarting;
        case ISHSandboxLifecycleStarting:
            return to == ISHSandboxLifecycleRunning || to == ISHSandboxLifecycleFailed;
        case ISHSandboxLifecycleRunning:
            return to == ISHSandboxLifecycleQuiescing || to == ISHSandboxLifecycleStopping || to == ISHSandboxLifecycleFailed;
        case ISHSandboxLifecycleQuiescing:
            return to == ISHSandboxLifecycleQuiesced || to == ISHSandboxLifecycleFailed;
        case ISHSandboxLifecycleQuiesced:
            return to == ISHSandboxLifecycleStarting || to == ISHSandboxLifecycleIdle;
        case ISHSandboxLifecycleStopping:
            return to == ISHSandboxLifecycleIdle || to == ISHSandboxLifecycleFailed;
        case ISHSandboxLifecycleFailed:
            return to == ISHSandboxLifecycleStarting || to == ISHSandboxLifecycleIdle;
    }
    return NO;
}

- (void)transitionToState:(ISHSandboxLifecycleState)state errorCode:(NSString *_Nullable)errorCode {
    ISHSandboxLifecycleState oldState = self.lifecycleState;
    if (oldState == state) return;

    if (![self isValidTransitionFrom:oldState to:state]) {
        NSLog(@"[IOSSandboxBridge] invalid lifecycle transition: %@ -> %@",
              kIOSSandboxLifecycleStateName[oldState],
              kIOSSandboxLifecycleStateName[state]);
        self.lastErrorCode = @"ISH_INVALID_LIFECYCLE_TRANSITION";
        return;
    }

    self.lifecycleState = state;
    self.lastTransitionAt = [self now];
    self.lastErrorCode = errorCode;

    switch (state) {
        case ISHSandboxLifecycleIdle:
        case ISHSandboxLifecycleQuiesced:
        case ISHSandboxLifecycleStarting:
        case ISHSandboxLifecycleQuiescing:
        case ISHSandboxLifecycleStopping:
            self.availability = ISHAvailabilityAvailable;
            break;
        case ISHSandboxLifecycleRunning:
            self.availability = ISHAvailabilityRunning;
            self.lastStableAt = self.lastTransitionAt;
            break;
        case ISHSandboxLifecycleFailed:
            self.availability = ISHAvailabilityError;
            break;
    }
}

#pragma mark - Availability Mapping

- (ISHAvailability)availability {
    switch (self.lifecycleState) {
        case ISHSandboxLifecycleIdle:        return ISHAvailabilityAvailable;
        case ISHSandboxLifecycleStarting:    return ISHAvailabilityStarting;
        case ISHSandboxLifecycleRunning:     return ISHAvailabilityRunning;
        case ISHSandboxLifecycleQuiescing:   return ISHAvailabilityAvailable;
        case ISHSandboxLifecycleQuiesced:    return ISHAvailabilityAvailable;
        case ISHSandboxLifecycleStopping:    return ISHAvailabilityAvailable;
        case ISHSandboxLifecycleFailed:      return ISHAvailabilityError;
    }
    return ISHAvailabilityUnavailable;
}

#pragma mark - Start

- (BOOL)startWithConfig:(ISHBridgeConfig *)config error:(NSError *_Nullable *_Nullable)error {
    __block BOOL success = NO;
    __block NSError *localError = nil;
    dispatch_sync(self.lifecycleQueue, ^{
        LifecycleStartResult result = [self startInternalWithConfig:config error:&localError];
        success = (result != LifecycleStartResultStartInProgress && localError == nil);
    });
    if (error && localError) {
        *error = localError;
    }
    return success;
}

- (void)startWithConfig:(ISHBridgeConfig *)config
             completion:(void (^)(BOOL, NSError *_Nullable))completion {
    dispatch_async(self.lifecycleQueue, ^{
        NSError *err = nil;
        LifecycleStartResult result = [self startInternalWithConfig:config error:&err];
        if (result == LifecycleStartResultStartInProgress || err) {
            dispatch_async(dispatch_get_main_queue(), ^{
                if (completion) completion(NO, err);
            });
            return;
        }
        dispatch_async(dispatch_get_main_queue(), ^{
            if (completion) completion(YES, nil);
        });
    });
}

- (LifecycleStartResult)startInternalWithConfig:(ISHBridgeConfig *)config
                                          error:(NSError *_Nullable *_Nullable)error {
    if (self.lifecycleState == ISHSandboxLifecycleRunning) {
        if ([self.currentConfig.rootfsURI isEqualToString:config.rootfsURI] || (config.rootfsURI.length == 0 && self.runningRootfsPath)) {
            self.desiredRunning = YES;
            return LifecycleStartResultAlreadyRunning;
        }
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                        code:IOSSandboxBridgeErrorRestartRequired
                                    userInfo:@{NSLocalizedDescriptionKey: @"start with different rootfs requires restart"}];
        }
        self.restartRequired = YES;
        return LifecycleStartResultStartInProgress;
    }

    if (self.lifecycleState == ISHSandboxLifecycleStarting) {
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                        code:IOSSandboxBridgeErrorLifecycleStarting
                                    userInfo:@{NSLocalizedDescriptionKey: @"start already in progress"}];
        }
        return LifecycleStartResultStartInProgress;
    }

    if (self.lifecycleState != ISHSandboxLifecycleIdle &&
        self.lifecycleState != ISHSandboxLifecycleQuiesced &&
        self.lifecycleState != ISHSandboxLifecycleFailed) {
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                        code:IOSSandboxBridgeErrorLifecycleNotRunning
                                    userInfo:@{NSLocalizedDescriptionKey: @"cannot start from current state"}];
        }
        return LifecycleStartResultStartInProgress;
    }

    self.desiredRunning = YES;
    self.restartRequired = NO;
    self.recoveryPending = NO;
    self.currentConfig = config;
    [self transitionToState:ISHSandboxLifecycleStarting errorCode:nil];

    NSString *rootfsPath = [self resolveRootfsPathForConfig:config];
    if (!rootfsPath || rootfsPath.length == 0) {
        NSError *err = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                           code:IOSSandboxBridgeErrorRootfsNotReady
                                       userInfo:@{NSLocalizedDescriptionKey: @"no active rootfs available"}];
        [self transitionToState:ISHSandboxLifecycleFailed errorCode:@"ROOTFS_NOT_READY"];
        if (error) *error = err;
        return LifecycleStartResultStartInProgress;
    }

    NSError *startError = nil;
    BOOL ok = [self.runtime startWithRootfsPath:rootfsPath
                                       workdir:nil
                                   environment:config.environment
                                         error:&startError];
    if (!ok) {
        NSString *errCode = [self bridgeErrorCodeForNative:startError.code];
        [self transitionToState:ISHSandboxLifecycleFailed errorCode:errCode];
        if (error) {
            *error = startError ?: [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                                        code:IOSSandboxBridgeErrorRuntimeFailed
                                                    userInfo:@{NSLocalizedDescriptionKey: @"iSH kernel failed to start"}];
        }
        return LifecycleStartResultStartInProgress;
    }

    self.ishInitialized = YES;
    self.generation++;
    self.runningRootfsPath = rootfsPath;
    self.runningRootfsVersion = [self.resolver resolveCurrentRootfs].version ?: self.currentConfig.runtimeID;
    self.runningRootfsDigest = [self.resolver resolveCurrentRootfs].digestSHA256;
    self.activeExecutionID = nil;

    [self transitionToState:ISHSandboxLifecycleRunning errorCode:nil];
    return LifecycleStartResultStarted;
}

- (NSString *)resolveRootfsPathForConfig:(ISHBridgeConfig *)config {
    if (config.rootfsURI.length > 0) {
        if ([config.rootfsURI hasPrefix:@"/"]) {
            return config.rootfsURI;
        }
    }
    RootfsDescriptor *active = [self.resolver resolveCurrentRootfs];
    if (active && active.mountURL) {
        return active.mountURL.path;
    }
    return config.rootfsURI;
}

#pragma mark - Stop

- (void)stop {
    dispatch_sync(self.lifecycleQueue, ^{
        [self stopInternalWithReason:@"user_stop" completion:nil];
    });
}

- (void)stopWithCompletion:(void (^)(BOOL, NSError *_Nullable))completion {
    dispatch_async(self.lifecycleQueue, ^{
        [self stopInternalWithReason:@"user_stop" completion:completion];
    });
}

- (void)stopInternalWithReason:(NSString *)reason
                    completion:(void (^)(BOOL, NSError *_Nullable))completion {
    if (self.lifecycleState == ISHSandboxLifecycleIdle ||
        self.lifecycleState == ISHSandboxLifecycleQuiesced) {
        self.desiredRunning = NO;
        if (completion) {
            dispatch_async(dispatch_get_main_queue(), ^{
                completion(YES, nil);
            });
        }
        return;
    }

    if (self.lifecycleState == ISHSandboxLifecycleStopping) {
        self.pendingStopReason = reason;
        self.pendingCompletion = completion;
        return;
    }

    if (self.lifecycleState != ISHSandboxLifecycleRunning &&
        self.lifecycleState != ISHSandboxLifecycleQuiescing &&
        self.lifecycleState != ISHSandboxLifecycleFailed) {
        NSError *err = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                           code:IOSSandboxBridgeErrorLifecycleNotRunning
                                       userInfo:@{NSLocalizedDescriptionKey: @"cannot stop from current state"}];
        if (completion) {
            dispatch_async(dispatch_get_main_queue(), ^{
                completion(NO, err);
            });
        }
        return;
    }

    self.desiredRunning = NO;
    self.resumeOnForeground = NO;
    [self transitionToState:ISHSandboxLifecycleStopping errorCode:nil];

    [self drainActiveExecutionWithReason:reason completion:^(BOOL drained) {
        NSError *stopError = nil;
        [self.runtime stop];
        self.ishInitialized = NO;
        self.activeExecutionID = nil;
        self.runningRootfsPath = nil;
        self.runningRootfsVersion = nil;
        self.runningRootfsDigest = nil;

        if (!drained) {
            stopError = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                            code:IOSSandboxBridgeErrorRuntimeFailed
                                        userInfo:@{NSLocalizedDescriptionKey: @"stop completed with pending execution"}];
            [self transitionToState:ISHSandboxLifecycleFailed errorCode:@"EXEC_DRAIN_FAILED"];
        } else {
            [self transitionToState:ISHSandboxLifecycleIdle errorCode:nil];
        }

        if (completion) {
            dispatch_async(dispatch_get_main_queue(), ^{
                completion(drained, stopError);
            });
        }
    }];
}

#pragma mark - Execute

- (ISHBridgeResult *)executeCommand:(ISHBridgeCommand *)command
                              error:(NSError *_Nullable *_Nullable)error {
    __block ISHBridgeResult *result = [[ISHBridgeResult alloc] init];
    __block NSError *localError = nil;

    dispatch_sync(self.lifecycleQueue, ^{
        if (self.lifecycleState != ISHSandboxLifecycleRunning) {
            NSString *code = [self lifecycleStateForExecuteError];
            localError = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                             code:[self bridgeErrorCodeForLifecycleState]
                                         userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"sandbox not running, state: %@", kIOSSandboxLifecycleStateName[self.lifecycleState]]}];
            result.stale = YES;
            self.lastErrorCode = code;
            return;
        }

        result.generation = self.generation;

        NSError *execError = nil;
        AmitiaISHExecutionResult *native = [self.runtime executeCommand:command.command
                                                                  stdin:command.stdin
                                                               workdir:command.workDir
                                                               timeout:command.timeout
                                                                 error:&execError];

        result.exitCode = native.exitCode;
        result.stdout = native.stdout ?: @"";
        result.stderr = native.stderr ?: @"";
        result.error = native.errorMessage;

        if (native.generation != 0 && native.generation != self.generation) {
            result.stale = YES;
            result.error = @"STALE_EXECUTION_RESULT";
            self.lastErrorCode = @"ISH_STALE_EXECUTION_RESULT";
        }

        if (execError) {
            localError = execError;
            if ([self isRuntimeFatal:execError]) {
                self.recoveryPending = YES;
                self.lastErrorCode = [self bridgeErrorCodeForNative:execError.code];
                [self transitionToState:ISHSandboxLifecycleFailed errorCode:self.lastErrorCode];
                [self attemptRecoveryIfNeeded];
            }
        }
    });

    if (error && localError) {
        *error = localError;
    }
    return result;
}

#pragma mark - Restart

- (void)restartWithReason:(NSString *)reason
               completion:(void (^)(BOOL, NSError *_Nullable))completion {
    dispatch_async(self.lifecycleQueue, ^{
        self.desiredRunning = YES;
        self.restartRequired = NO;
        self.pendingRestartReason = reason ?: @"explicit_restart";

        void (^originalCompletion)(BOOL, NSError *) = completion;

        [self stopInternalWithReason:self.pendingRestartReason completion:^(BOOL stopped, NSError *stopError) {
            if (!stopped && self.lifecycleState != ISHSandboxLifecycleIdle &&
                self.lifecycleState != ISHSandboxLifecycleQuiesced) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    if (originalCompletion) originalCompletion(NO, stopError);
                });
                return;
            }

            NSError *startErr = nil;
            LifecycleStartResult startResult = [self startInternalWithConfig:self.currentConfig error:&startErr];
            if (startResult == LifecycleStartResultStartInProgress || startErr) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    if (originalCompletion) originalCompletion(NO, startErr);
                });
                return;
            }
            dispatch_async(dispatch_get_main_queue(), ^{
                if (originalCompletion) originalCompletion(YES, nil);
            });
        }];
    });
}

#pragma mark - Execute Failure Classification

- (BOOL)isRuntimeFatal:(NSError *)error {
    if (!error) return NO;
    if (error.domain != kAmitiaISHRuntimeErrorDomain && error.domain != kIOSSandboxBridgeErrorDomain) return NO;
    NSInteger code = error.code;
    if (code == AmitiaISHRuntimeErrorCodeNotInitialized ||
        code == IOSSandboxBridgeErrorRuntimeFailed) {
        return YES;
    }
    return NO;
}

- (NSString *)lifecycleStateForExecuteError {
    switch (self.lifecycleState) {
        case ISHSandboxLifecycleStarting:    return @"ISH_LIFECYCLE_STARTING";
        case ISHSandboxLifecycleQuiescing:
        case ISHSandboxLifecycleQuiesced:    return @"ISH_LIFECYCLE_QUIESCED";
        case ISHSandboxLifecycleStopping:    return @"ISH_LIFECYCLE_STOPPING";
        case ISHSandboxLifecycleFailed:      return @"ISH_RUNTIME_FAILED";
        case ISHSandboxLifecycleIdle:        return @"ISH_LIFECYCLE_NOT_RUNNING";
        default:                             return @"ISH_LIFECYCLE_UNKNOWN";
    }
}

- (NSInteger)bridgeErrorCodeForLifecycleState {
    switch (self.lifecycleState) {
        case ISHSandboxLifecycleStarting:    return IOSSandboxBridgeErrorLifecycleStarting;
        case ISHSandboxLifecycleQuiescing:
        case ISHSandboxLifecycleQuiesced:    return IOSSandboxBridgeErrorLifecycleQuiesced;
        case ISHSandboxLifecycleStopping:    return IOSSandboxBridgeErrorLifecycleStopping;
        case ISHSandboxLifecycleFailed:      return IOSSandboxBridgeErrorRuntimeFailed;
        default:                             return IOSSandboxBridgeErrorLifecycleNotRunning;
    }
}

- (NSString *)bridgeErrorCodeForNative:(NSInteger)code {
    switch (code) {
        case AmitiaISHRuntimeErrorCodeNotInitialized:     return @"ISH_KERNEL_NOT_RUNNING";
        case AmitiaISHRuntimeErrorCodeInternal:           return @"ISH_INTERNAL_RUNTIME_FAULT";
        case IOSSandboxBridgeErrorRuntimeFailed:          return @"ISH_RUNTIME_FAILED";
        default:                                          return @"ISH_RUNTIME_FAILED";
    }
}

#pragma mark - Active Execution Handling

- (void)drainActiveExecutionWithReason:(NSString *)reason
                            completion:(void (^)(BOOL drained))completion {
    if (completion) completion(YES);
}

#pragma mark - Recovery

- (void)attemptRecoveryIfNeeded {
    if (!self.recoveryPending) return;
    if (!self.desiredRunning) {
        self.recoveryPending = NO;
        return;
    }

    NSDate *now = [self now];
    NSTimeInterval window = kRecoveryWindowSeconds;

    NSMutableArray *recent = [NSMutableArray array];
    for (NSDate *attempt in self.recoveryAttempts) {
        if ([now timeIntervalSinceDate:attempt] <= window) {
            [recent addObject:attempt];
        }
    }
    [self.recoveryAttempts setArray:recent];

    if (self.recoveryAttempts.count >= kRecoveryMaxAttempts) {
        self.recoveryPending = NO;
        return;
    }

    RootfsDescriptor *active = [self.resolver resolveCurrentRootfs];
    if (!active || active.state != RootfsStateInstalled) {
        self.recoveryPending = NO;
        self.lastErrorCode = @"ROOTFS_INVALID";
        return;
    }

    NSTimeInterval delay = (self.recoveryAttempts.count < 3)
        ? kRecoveryDelays[self.recoveryAttempts.count]
        : kRecoveryDelays[2];
    [self.recoveryAttempts addObject:now];

    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(delay * NSEC_PER_SEC)),
                   self.recoveryQueue, ^{
        [self performRecovery];
    });
}

- (void)performRecovery {
    dispatch_sync(self.lifecycleQueue, ^{
        if (!self.recoveryPending || !self.desiredRunning) {
            self.recoveryPending = NO;
            return;
        }
        self.recoveryPending = NO;
        self.restartRequired = YES;
        [self stopInternalWithReason:@"recovery" completion:nil];
    });

    dispatch_async(self.lifecycleQueue, ^{
        NSError *err = nil;
        LifecycleStartResult result = [self startInternalWithConfig:self.currentConfig error:&err];
        if (result == LifecycleStartResultStartInProgress || err) {
            if (self.recoveryAttempts.count < kRecoveryMaxAttempts) {
                self.recoveryPending = YES;
            }
        }
    });
}

- (void)resetRecoveryBudgetIfNeeded {
    if (!self.lastStableAt) return;
    NSTimeInterval elapsed = [[self now] timeIntervalSinceDate:self.lastStableAt];
    if (elapsed >= kStableWindowSeconds) {
        [self.recoveryAttempts removeAllObjects];
    }
}

#pragma mark - Lifecycle: Background / Foreground

- (void)applicationDidEnterBackground {
    dispatch_async(self.lifecycleQueue, ^{
        if (self.lifecycleState == ISHSandboxLifecycleRunning) {
            self.resumeOnForeground = self.desiredRunning;
            [self quiesceForBackground];
        }
    });
}

- (void)quiesceForBackground {
    [self transitionToState:ISHSandboxLifecycleQuiescing errorCode:nil];
    [self drainActiveCompletion:^(BOOL drained) {
        [self.runtime stop];
        self.ishInitialized = NO;
        self.activeExecutionID = nil;
        self.runningRootfsPath = nil;
        [self transitionToState:ISHSandboxLifecycleQuiesced errorCode:nil];
    }];
}

- (void)drainActiveCompletion:(void (^)(BOOL drained))handler {
    if (handler) handler(YES);
}

- (void)applicationWillEnterForeground {
    dispatch_async(self.lifecycleQueue, ^{
        if (self.lifecycleState != ISHSandboxLifecycleQuiesced) return;

        RootfsDescriptor *active = [self.resolver resolveCurrentRootfs];
        if (!active || active.state != RootfsStateInstalled) {
            [self transitionToState:ISHSandboxLifecycleFailed errorCode:@"ROOTFS_INVALID"];
            self.lastErrorCode = @"ROOTFS_INVALID";
            self.resumeOnForeground = NO;
            return;
        }

        if (self.resumeOnForeground && self.desiredRunning) {
            if (self.runningRootfsDigest && active.digestSha256 &&
                ![self.runningRootfsDigest isEqualToString:active.digestSha256]) {
                self.restartRequired = YES;
            }
            NSError *err = nil;
            LifecycleStartResult result = [self startInternalWithConfig:self.currentConfig error:&err];
            if (result == LifecycleStartResultStartInProgress) {
                self.lastErrorCode = @"FOREGROUND_RESUME_FAILED";
            }
        } else {
            [self transitionToState:ISHSandboxLifecycleIdle errorCode:nil];
        }
    });
}

- (void)applicationWillTerminate {
    dispatch_sync(self.lifecycleQueue, ^{
        [self.runtime stop];
        [self transitionToState:ISHSandboxLifecycleIdle errorCode:nil];
    });
}

#pragma mark - Health

- (ISHBridgeHealth *)health {
    __block ISHBridgeHealth *h;
    dispatch_sync(self.lifecycleQueue, ^{
        h = [self collectHealth];
    });
    return h;
}

- (ISHBridgeHealth *)collectHealth {
    ISHBridgeHealth *h = [[ISHBridgeHealth alloc] init];
    h.healthy = (self.lifecycleState == ISHSandboxLifecycleRunning);
    h.ishInitialized = self.ishInitialized && (self.lifecycleState == ISHSandboxLifecycleRunning);
    h.message = [self messageForState:self.lifecycleState];

    RootfsDescriptor *active = [self.resolver resolveCurrentRootfs];
    h.rootfsInstalled = (active != nil && active.state == RootfsStateInstalled);

    h.lifecycleState = self.lifecycleState;
    h.generation = self.generation;
    h.desiredRunning = self.desiredRunning;
    h.restartRequired = self.restartRequired;
    h.recoveryPending = self.recoveryPending;
    h.activeExecutionID = self.activeExecutionID;
    h.runningRootfsVersion = self.runningRootfsVersion;
    h.runningRootfsDigest = self.runningRootfsDigest;
    h.lastErrorCode = self.lastErrorCode;
    h.lastTransitionAt = self.lastTransitionAt;
    return h;
}

- (NSString *)messageForState:(ISHSandboxLifecycleState)state {
    switch (state) {
        case ISHSandboxLifecycleIdle:        return @"sandbox idle";
        case ISHSandboxLifecycleStarting:    return @"sandbox starting";
        case ISHSandboxLifecycleRunning:     return @"sandbox running";
        case ISHSandboxLifecycleQuiescing:   return @"sandbox quiescing";
        case ISHSandboxLifecycleQuiesced:    return @"sandbox quiesced";
        case ISHSandboxLifecycleStopping:    return @"sandbox stopping";
        case ISHSandboxLifecycleFailed:      return @"sandbox failed";
    }
    return @"sandbox unknown";
}

@end

#pragma mark - Implementations

@implementation ISHBridgeConfig
@end

@implementation ISHBridgeCommand
@end

@implementation ISHBridgeResult
@end

@implementation ISHBridgeHealth
@end
