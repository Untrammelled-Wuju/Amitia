#import <Foundation/Foundation.h>
#import "RootfsDescriptor.h"
#import "RootfsResolver.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsInstallStep) {
    RootfsInstallStepPending = 0,
    RootfsInstallStepValidating,
    RootfsInstallStepResolvingSource,
    RootfsInstallStepDownloading,
    RootfsInstallStepVerifying,
    RootfsInstallStepStaging,
    RootfsInstallStepExtracting,
    RootfsInstallStepValidatingPackage,
    RootfsInstallStepPreparingTarget,
    RootfsInstallStepAtomicMove,
    RootfsInstallStepWritingManifest,
    RootfsInstallStepAtomicActivation,
    RootfsInstallStepCleanup,
    RootfsInstallStepComplete,
    RootfsInstallStepFailed
};

typedef void(^RootfsInstallProgressBlock)(RootfsInstallStep step, double fraction, NSString *_Nullable message);

@interface RootfsInstallResult : NSObject
@property (nonatomic, strong, readonly) RootfsDescriptor *descriptor;
@property (nonatomic, readonly) BOOL activated;
@property (nonatomic, readonly) BOOL requiresSandboxRestart;
- (instancetype)initWithDescriptor:(RootfsDescriptor *)descriptor activated:(BOOL)activated requiresRestart:(BOOL)restart;
@end

@interface RootfsInstallRequest : NSObject
@property (nonatomic, copy) NSString *version;
@property (nonatomic, copy) NSString *architecture;
@property (nonatomic, copy) NSString *expectedDigestSHA256;
@property (nonatomic, copy, nullable) NSURL *remoteURL;
@property (nonatomic, copy, nullable) NSURL *localBundleURL;
@property (nonatomic) BOOL allowCellularDownload;
@property (nonatomic) BOOL forceReplace;
@property (nonatomic) int64_t maxArchiveBytes;
@property (nonatomic, copy, nullable) NSString *packageFormat;
- (instancetype)init;
@end

@interface RootfsInstaller : NSObject

@property (nonatomic, strong, readonly) RootfsResolver *resolver;
@property (nonatomic, readonly) BOOL isInstalling;
@property (nonatomic, copy, readonly, nullable) NSString *currentInstallID;
@property (nonatomic, readonly) BOOL needsStoragePreflight;

- (instancetype)initWithResolver:(RootfsResolver *)resolver;

- (void)installRootfsWithRequest:(RootfsInstallRequest *)request
                        progress:(nullable RootfsInstallProgressBlock)progress
                      completion:(void(^)(BOOL success, RootfsInstallResult *_Nullable result, NSError *_Nullable error))completion;

- (void)cancelInstallation;

- (BOOL)verifyInstalledRootfs:(RootfsDescriptor *)descriptor error:(NSError *_Nullable *_Nullable)error;
- (BOOL)deactivateRootfsVersion:(NSString *)version architecture:(NSString *)architecture error:(NSError *_Nullable *_Nullable)error;
- (BOOL)isRootfsActive;

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
    RootfsInstallerErrorConcurrentInstallation,
    RootfsInstallerErrorUnsupportedFormat,
    RootfsInstallerErrorManifestInvalid,
    RootfsInstallerErrorVersionConflict,
    RootfsInstallerErrorArchiveTooLarge,
    RootfsInstallerErrorExtractedSizeExceeded,
    RootfsInstallerErrorTooManyEntries,
    RootfsInstallerErrorRemoteRedirectRejected,
    RootfsInstallerErrorDigestMalformed
};

NS_ASSUME_NONNULL_END
