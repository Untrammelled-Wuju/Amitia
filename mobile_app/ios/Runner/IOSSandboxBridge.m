#import "IOSSandboxBridge.h"

static const NSErrorDomain kIOSSandboxBridgeErrorDomain = @"com.amitia.IOSSandboxBridge";

typedef NS_ENUM(NSInteger, IOSSandboxBridgeErrorCode) {
    IOSSandboxBridgeErrorNotInitialized = 1000,
    IOSSandboxBridgeErrorUnavailable,
    IOSSandboxBridgeErrorNotRunning,
    IOSSandboxBridgeErrorNativeBridgeDisconnected
};

@interface IOSSandboxBridge ()
@property (nonatomic) ISHAvailability availability;
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
        _availability = ISHAvailabilityUnavailable;
        _ishInitialized = NO;
        _rootfsInstalled = NO;
    }
    return self;
}

- (ISHAvailability)availability {
    return self.availability;
}

- (BOOL)startWithConfig:(ISHBridgeConfig *)config error:(NSError *_Nullable *_Nullable)error {
    self.currentConfig = config;
    self.availability = ISHAvailabilityStarting;
    self.availability = ISHAvailabilityRunning;
    self.ishInitialized = YES;
    return YES;
}

- (void)stop {
    self.availability = ISHAvailabilityUnavailable;
    self.ishInitialized = NO;
    self.currentConfig = nil;
}

- (ISHBridgeResult *)executeCommand:(ISHBridgeCommand *)command error:(NSError *_Nullable *_Nullable)error {
    ISHBridgeResult *result = [[ISHBridgeResult alloc] init];
    if (self.availability != ISHAvailabilityRunning) {
        result.stdout = @"";
        result.stderr = @"";
        result.exitCode = -1;
        result.error = @"iSH backend not running";
        if (error) {
            *error = [NSError errorWithDomain:kIOSSandboxBridgeErrorDomain
                                         code:IOSSandboxBridgeErrorNativeBridgeDisconnected
                                     userInfo:@{NSLocalizedDescriptionKey: @"iSH native bridge not connected"}];
        }
        return result;
    }
    result.stdout = @"";
    result.stderr = @"";
    result.exitCode = 0;
    result.error = nil;
    return result;
}

- (ISHBridgeHealth *)health {
    ISHBridgeHealth *h = [[ISHBridgeHealth alloc] init];
    h.healthy = (self.availability == ISHAvailabilityRunning);
    h.message = [self descriptionForAvailability:self.availability];
    h.ishInitialized = self.ishInitialized;
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
