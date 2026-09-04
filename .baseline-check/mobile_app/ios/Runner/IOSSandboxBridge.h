#import <Foundation/Foundation.h>
#import "RootfsResolver.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, ISHAvailability) {
    ISHAvailabilityUnavailable = 0,
    ISHAvailabilityAvailable,
    ISHAvailabilityStarting,
    ISHAvailabilityRunning,
    ISHAvailabilityError
};

typedef NS_ENUM(NSInteger, ISHSandboxLifecycleState) {
    ISHSandboxLifecycleIdle = 0,
    ISHSandboxLifecycleStarting,
    ISHSandboxLifecycleRunning,
    ISHSandboxLifecycleQuiescing,
    ISHSandboxLifecycleQuiesced,
    ISHSandboxLifecycleStopping,
    ISHSandboxLifecycleFailed
};

extern NSString * const kIOSSandboxLifecycleStateName[];
extern NSString * const kIOSSandboxBridgeErrorDomain;
extern NSString * const kAmitiaISHRuntimeErrorDomain;

typedef NS_ENUM(NSInteger, IOSSandboxBridgeErrorCode) {
    IOSSandboxBridgeErrorNotInitialized = 1000,
    IOSSandboxBridgeErrorUnavailable,
    IOSSandboxBridgeErrorNotRunning,
    IOSSandboxBridgeErrorNativeBridgeDisconnected,
    IOSSandboxBridgeErrorRootfsNotReady,
    IOSSandboxBridgeErrorExecFailed,
    IOSSandboxBridgeErrorLifecycleNotRunning,
    IOSSandboxBridgeErrorLifecycleStarting,
    IOSSandboxBridgeErrorLifecycleStopping,
    IOSSandboxBridgeErrorLifecycleQuiesced,
    IOSSandboxBridgeErrorRestartRequired,
    IOSSandboxBridgeErrorRuntimeFailed,
    IOSSandboxBridgeErrorStaleExecutionResult,
    IOSSandboxBridgeErrorInvalidLifecycleTransition,
};

typedef NS_ENUM(NSInteger, IOSSandboxLifecycleRecoveryErrorKind) {
    IOSSandboxLifecycleRecoveryErrorKindCommand = 0,
    IOSSandboxLifecycleRecoveryErrorKindFatal,
    IOSSandboxLifecycleRecoveryErrorKindRootFsInvalid,
    IOSSandboxLifecycleRecoveryErrorKindUserStopped,
    IOSSandboxLifecycleRecoveryErrorKindBackground,
    IOSSandboxLifecycleRecoveryErrorKindArchitecture,
};

extern NSString * const kRootfsActiveVersionDidChangeNotification;

@class AmitiaISHRuntime;

@interface ISHBridgeConfig : NSObject
@property (nonatomic, copy, nullable) NSString *runtimeID;
@property (nonatomic, copy, nullable) NSString *workspaceURI;
@property (nonatomic, copy, nullable) NSString *rootfsURI;
@property (nonatomic, copy, nullable) NSDictionary<NSString *, NSString *> *environment;
@end

@interface ISHBridgeCommand : NSObject
@property (nonatomic, copy) NSArray<NSString *> *command;
@property (nonatomic, copy, nullable) NSString *stdin;
@property (nonatomic) NSInteger timeout;
@property (nonatomic, copy, nullable) NSString *workDir;
@end

@interface ISHBridgeResult : NSObject
@property (nonatomic, copy) NSString *stdout;
@property (nonatomic, copy) NSString *stderr;
@property (nonatomic) int64_t exitCode;
@property (nonatomic, copy, nullable) NSString *error;
@property (nonatomic, copy, nullable) NSString *executionID;
@property (nonatomic) uint64_t generation;
@property (nonatomic) BOOL stale;
@end

@interface ISHBridgeHealth : NSObject
@property (nonatomic) BOOL healthy;
@property (nonatomic, copy) NSString *message;
@property (nonatomic) BOOL ishInitialized;
@property (nonatomic) BOOL rootfsInstalled;

@property (nonatomic) ISHSandboxLifecycleState lifecycleState;
@property (nonatomic) uint64_t generation;
@property (nonatomic) BOOL desiredRunning;
@property (nonatomic) BOOL restartRequired;
@property (nonatomic) BOOL recoveryPending;
@property (nonatomic, copy, nullable) NSString *activeExecutionID;
@property (nonatomic, copy, nullable) NSString *runningRootfsVersion;
@property (nonatomic, copy, nullable) NSString *runningRootfsDigest;
@property (nonatomic, copy, nullable) NSString *lastErrorCode;
@property (nonatomic, strong, nullable) NSDate *lastTransitionAt;
@end

@interface IOSSandboxBridge : NSObject

@property (nonatomic, strong, readonly) AmitiaISHRuntime *runtime;
@property (nonatomic, strong, readonly, nullable) RootfsResolver *resolver;

+ (instancetype)shared;
+ (instancetype)sharedWithResolver:(nullable RootfsResolver *)resolver;

- (ISHAvailability)availability;
- (ISHBridgeHealth *)health;

- (BOOL)startWithConfig:(ISHBridgeConfig *)config error:(NSError *_Nullable *_Nullable)error;
- (void)stop;

- (void)startWithConfig:(ISHBridgeConfig *)config
             completion:(void (^)(BOOL success, NSError *_Nullable error))completion;
- (void)stopWithCompletion:(void (^)(BOOL success, NSError *_Nullable error))completion;
- (void)restartWithReason:(nullable NSString *)reason
               completion:(void (^)(BOOL success, NSError *_Nullable error))completion;

- (ISHBridgeResult *)executeCommand:(ISHBridgeCommand *)command error:(NSError *_Nullable *_Nullable)error;

- (void)applicationDidEnterBackground;
- (void)applicationWillEnterForeground;
- (void)applicationWillTerminate;

@end

NS_ASSUME_NONNULL_END
