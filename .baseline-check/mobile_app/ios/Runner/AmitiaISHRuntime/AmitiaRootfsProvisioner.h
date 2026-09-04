#import <Foundation/Foundation.h>
#import "AmitiaRootfsCatalog.h"
#import "AmitiaRootfsStore.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, AmitiaRootfsProgressPhase) {
    AmitiaRootfsProgressPending = 0,
    AmitiaRootfsProgressDownloading,
    AmitiaRootfsProgressVerifying,
    AmitiaRootfsProgressPreflighting,
    AmitiaRootfsProgressExtracting,
    AmitiaRootfsProgressValidating,
    AmitiaRootfsProgressCommitting,
    AmitiaRootfsProgressActivating,
    AmitiaRootfsProgressCompleted,
    AmitiaRootfsProgressFailed,
    AmitiaRootfsProgressCancelled,
};

@interface AmitiaRootfsProgress : NSObject
@property (nonatomic, copy, readonly) NSString *installationId;
@property (nonatomic, readonly) AmitiaRootfsProgressPhase phase;
@property (nonatomic, readonly) int64_t bytesWritten;
@property (nonatomic, readonly) int64_t totalBytes;
@property (nonatomic, copy, readonly, nullable) NSString *message;
@property (nonatomic, readonly) BOOL done;
@property (nonatomic, readonly) BOOL failed;
@end

typedef void (^AmitiaRootfsProgressBlock)(AmitiaRootfsProgress *progress);
typedef void (^AmitiaRootfsCompletionBlock)(NSString *_Nullable installationId, NSError *_Nullable error);

@interface AmitiaRootfsProvisioner : NSObject

@property (nonatomic, copy, readonly) NSString *guestArchitecture;
@property (nonatomic, copy, readonly) NSString *baseDirectory;

- (instancetype)initWithCatalog:(AmitiaRootfsCatalog *)catalog
                      store:(AmitiaRootfsStore *)store
            guestArchitecture:(NSString *)guestArch NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

- (NSDictionary<NSString *, id> *)statusDictionary;

- (void)ensureRootfs:(AmitiaRootfsCatalogEntry *)entry
     activateAfterInstall:(BOOL)activate
                progress:(AmitiaRootfsProgressBlock)progress
             completion:(AmitiaRootfsCompletionBlock)completion;

- (BOOL)activateVersion:(NSString *)installationId error:(NSError **)error;
- (BOOL)repairVersion:(NSString *)installationId error:(NSError **)error;
- (BOOL)cancelInstall:(NSString *)installationId error:(NSError **)error;

@end

NS_ASSUME_NONNULL_END
