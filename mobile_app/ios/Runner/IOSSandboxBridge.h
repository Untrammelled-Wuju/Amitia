#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, ISHAvailability) {
    ISHAvailabilityUnavailable = 0,
    ISHAvailabilityAvailable,
    ISHAvailabilityStarting,
    ISHAvailabilityRunning,
    ISHAvailabilityError
};

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
@end

@interface ISHBridgeHealth : NSObject
@property (nonatomic) BOOL healthy;
@property (nonatomic, copy) NSString *message;
@property (nonatomic) BOOL ishInitialized;
@property (nonatomic) BOOL rootfsInstalled;
@end

@interface IOSSandboxBridge : NSObject

+ (instancetype)shared;

- (ISHAvailability)availability;
- (BOOL)startWithConfig:(ISHBridgeConfig *)config error:(NSError *_Nullable *_Nullable)error;
- (void)stop;
- (ISHBridgeResult *)executeCommand:(ISHBridgeCommand *)command error:(NSError *_Nullable *_Nullable)error;
- (ISHBridgeHealth *)health;

@end

NS_ASSUME_NONNULL_END
