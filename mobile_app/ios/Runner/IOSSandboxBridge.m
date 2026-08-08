#import "IOSSandboxBridge.h"
#import "AmitiaISHRuntime.h"

static const NSErrorDomain kIOSSandboxBridgeErrorDomain = @"com.amitia.IOSSandboxBridge";

typedef NS_ENUM(NSInteger, IOSSandboxBridgeErrorCode) {
    IOSSandboxBridgeErrorNotInitialized = 1000,
    IOSSandboxBridgeErrorUnavailable,
    IOSSandboxBridgeErrorNotRunning,
    IOSSandboxBridgeErrorNativeBridgeDisconnected,
    IOSSandboxBridgeErrorRootfsNotReady,
    IOSSandboxBridgeErrorExecFailed,
};

@interface IOSSandboxBridge ()
@property (nonatomic) ISHAvailability state;
@property (nonatomic, strong, nullable) ISHBridgeConfig *currentConfig;
@property (nonatomic) BOOL ishInitialized;
@property (nonatomic) BOOL rootfsInstalled;
@end

@implementation IOSSandboxBridge

+ (instancetype)shared {
    static IOSSandboxBridge *instance;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        instance = [[IOSSandboxBridge alloc] init];
    });
    return instance;
}

- (instancetype)init {
    self = [super init];
    if (self) {
        _runtime = [AmitiaISHRuntime shared];
        _state = ISHAvailabilityUnavailable;
        _ishInitialized = NO;
        _rootfsInstalled = NO;
    }
    return self;
}

- (ISHAvailability)availability {
    switch (self.runtime.state) {
        case AmitiaISHStateUnavailable: return ISHAvailabilityUnavailable;
        case AmitiaISHStateAvailable: return ISHAvailabilityAvailable;
        case AmitiaISHStateStarting: return ISHAvailabilityStarting;
        case AmitiaISHStateRunning: return ISHAvailabilityRunning;
        case AmitiaISHStateError: return ISHAvailabilityError;
    }
    return ISHAvailabilityUnavailable;
}

- (BOOL)startWithConfig:(ISHBridgeConfig *)config error:(NSError *_Nullable *_Nullable) error {
    self.currentConfig = config;

    if (self.runtime.state == AmitiaISHStateRunning) {
        self.state = ISHAvailabilityRunning;
        self.ishInitialized = YES;
        return YES;
    }

    NSString *rootfsPath = config.rootfsURI;
    if (!rootfsPath || rootfsPath.length == 0) {
        self.state = ISHAvailabilityError;
        self.ishInitialized = NO;
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                        code:IOSSandboxBridgeErrorRootfsNotReady
                                    userInfo:@{NSLocalizedDescriptionKey: @"rootfs path is empty"}];
        }
        return NO;
    }

    self.state = ISHAvailabilityStarting;

    NSError *startError = nil;
    BOOL ok = [self.runtime startWithRootfsPath:rootfsPath
                                       workdir:nil
                                   environment:config.environment
                                         error:&startError];
    if (ok) {
        self.state = ISHAvailabilityRunning;
        self.ishInitialized = YES;
        return YES;
    } else {
        self.state = ISHAvailabilityError;
        self.ishInitialized = NO;
        if (error) {
            *error = startError ?: [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                                        code:IOSSandboxBridgeErrorNotInitialized
                                                    userInfo:@{NSLocalizedDescriptionKey: @"iSH failed to start"}];
        }
        return NO;
    }
}

- (void)stop {
    [self.runtime stop];
    self.state = ISHAvailabilityUnavailable;
    self.ishInitialized = NO;
    self.currentConfig = nil;
}

- (ISHBridgeResult *)executeCommand:(ISHBridgeCommand *)command error:(NSError *_Nullable *_Nullable) error {
    ISHBridgeResult *result = [[ISHBridgeResult alloc] init];

    if (self.runtime.state != AmitiaISHStateRunning) {
        result.stdout = @"";
        result.stderr = @"";
        result.exitCode = -1;
        result.error = @"iSH runtime not running";
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                        code:IOSSandboxBridgeErrorNotRunning
                                    userInfo:@{NSLocalizedDescriptionKey: @"iSH kernel not running"}];
        }
        return result;
    }

    NSError *execError = nil;
    AmitiaISHExecutionResult *native = [self.runtime executeCommand:command.command
                                                              stdin:command.stdin
                                                           workdir:command.workDir
                                                           timeout:command.timeout
                                                             error:&execError];
    if (!native) {
        result.stdout = @"";
        result.stderr = @"";
        result.exitCode = -1;
        result.error = execError.localizedDescription ?: @"unknown execution error";
        if (error) {
            *error = execError;
        }
        return result;
    }

    result.stdout = native.stdout ?: @"";
    result.stderr = native.stderr ?: @"";
    result.exitCode = native.exitCode;
    result.error = native.errorMessage;

    return result;
}

- (ISHBridgeHealth *)health {
    ISHBridgeHealth *h = [[ISHBridgeHealth alloc] init];
    h.healthy = (self.runtime.state == AmitiaISHStateRunning);
    h.message = [self descriptionForAvailability:self.availability];
    h.ishInitialized = self.ishInitialized && (self.runtime.state == AmitiaISHStateRunning);
    h.rootfsInstalled = self.rootfsInstalled;
    return h;
}

- (NSString *)descriptionForAvailability:(ISHAvailability)state {
    switch (state) {
        case ISHAvailabilityUnavailable: return @"unavailable";
        case ISHAvailabilityAvailable: return @"available";
        case ISHAvailabilityStarting: return @"starting";
        case ISHAvailabilityRunning: return @"running";
        case ISHAvailabilityError: return @"error";
    }
    return @"unknown";
}

@end

@implementation ISHBridgeConfig
@end

@implementation ISHBridgeCommand
@end

@implementation ISHBridgeResult
@end

@implementation ISHBridgeHealth
@end
