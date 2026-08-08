#import <Foundation/Foundation.h>
#import "RootfsDescriptor.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsInstallStep) {
    RootfsInstallStepPending = 0,
    RootfsInstallStepDownloading,
    RootfsInstallStepVerifying,
    RootfsInstallStepExtracting,
    RootfsInstallStepValidating,
    RootfsInstallStepActivating,
    RootfsInstallStepComplete,
    RootfsInstallStepFailed
};

typedef void(^RootfsInstallProgressBlock)(RootfsInstallStep step, double fraction, NSString *_Nullable message);
typedef void(^RootfsInstallCompletionBlock)(BOOL success, RootfsDescriptor *_Nullable descriptor, NSError *_Nullable error);

@interface RootfsInstallRequest : NSObject
@property (nonatomic, copy) NSString *version;
@property (nonatomic, copy) NSString *architecture;
@property (nonatomic, copy) NSString *expectedDigestSHA256;
@property (nonatomic, copy, nullable) NSURL *remoteURL;
@property (nonatomic, copy, nullable) NSURL *localBundleURL;
@property (nonatomic) BOOL allowCellularDownload;
@end

@interface RootfsInstaller : NSObject

@property (nonatomic, strong, readonly) RootfsResolver *resolver;
@property (nonatomic, readonly) BOOL isInstalling;

- (instancetype)initWithResolver:(RootfsResolver *)resolver;

- (void)installRootfsWithRequest:(RootfsInstallRequest *)request
                        progress:(nullable RootfsInstallProgressBlock)progress
                      completion:(RootfsInstallCompletionBlock)completion;

- (void)cancelInstallation;

- (BOOL)verifyInstalledRootfs:(RootfsDescriptor *)descriptor error:(NSError *_Nullable *_Nullable)error;

- (BOOL)deactivateRootfsVersion:(NSString *)version architecture:(NSString *)architecture error:(NSError *_Nullable *_Nullable)error;

@end

extern NSErrorDomain const RootfsInstallerErrorDomain;

typedef NS_ERROR_ENUM(RootfsInstallerErrorDomain, RootfsInstallerError) {
    RootfsInstallerErrorInvalidRequest = 1000,
    RootfsInstallerErrorSourceUnavailable,
    RootfsInstallerErrorIntegrityMismatch,
    RootfsInstallerErrorExtractionFailed,
    RootfsInstallerErrorTraversalDetected,
    RootfsInstallerErrorSymlinkEscapeDetected,
    RootfsInstallerErrorLayoutInvalid,
    RootfsInstallerErrorArchitectureMismatch,
    RootfsInstallerErrorInsufficientStorage,
    RootfsInstallerErrorActivationFailed,
    RootfsInstallerErrorCancelled,
    RootfsInstallerErrorConcurrentInstallation
};

NS_ASSUME_NONNULL_END
