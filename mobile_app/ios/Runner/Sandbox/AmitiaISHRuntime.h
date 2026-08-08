#import <Foundation/Foundation.h>
#import "IOSSandboxBridge.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, AmitiaISHState) {
    AmitiaISHStateUnavailable = 0,
    AmitiaISHStateAvailable,
    AmitiaISHStateStarting,
    AmitiaISHStateRunning,
    AmitiaISHStateError
};

typedef NS_ENUM(NSInteger, AmitiaISHNativeError) {
    AmitiaISHNativeErrorNone = 0,
    AmitiaISHNativeErrorNotInitialized = -1,
    AmitiaISHNativeErrorRootfsNotReady = -2,
    AmitiaISHNativeErrorRootfsFormatUnsupported = -3,
    AmitiaISHNativeErrorInvalidArgument = -4,
    AmitiaISHNativeErrorExecFailed = -5,
    AmitiaISHNativeErrorExecTimeout = -6,
    AmitiaISHNativeErrorExecCancelled = -7,
    AmitiaISHNativeErrorExecBusy = -8,
    AmitiaISHNativeErrorInternal = -9,
};

@interface AmitiaISHExecutionResult : NSObject
@property (nonatomic, copy) NSString *stdout;
@property (nonatomic, copy) NSString *stderr;
@property (nonatomic) int exitCode;
@property (nonatomic) AmitiaISHNativeError errorCode;
@property (nonatomic, copy, nullable) NSString *errorMessage;
@property (nonatomic, copy, nullable) NSString *truncated;
@end

@interface AmitiaISHRuntime : NSObject

@property (nonatomic, readonly) AmitiaISHState state;

+ (instancetype)shared;

- (BOOL)startWithRootfsPath:(NSString *)rootfsPath
                   workdir:(nullable NSString *)workdir
                environment:(nullable NSDictionary<NSString *, NSString *> *)env
                      error:(NSError *_Nullable *_Nullable)error;

- (nullable AmitiaISHExecutionResult *)executeCommand:(NSArray<NSString *> *)argv
                                                 stdin:(nullable NSString *)stdin
                                              workdir:(nullable NSString *)workdir
                                              timeout:(NSInteger)timeout
                                                error:(NSError *_Nullable *_Nullable)error;

- (BOOL)cancelExecution:(NSString *)executionId error:(NSError *_Nullable *_Nullable)error;

- (void)stop;

@end

NS_ASSUME_NONNULL_END
