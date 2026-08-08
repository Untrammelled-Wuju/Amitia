#import "RootfsInstaller.h"
#import "RootfsIntegrityVerifier.h"

NSErrorDomain const RootfsInstallerErrorDomain = @"com.amitia.RootfsInstaller";

@interface RootfsInstaller ()
@property (nonatomic, strong, readwrite) RootfsResolver *resolver;
@property (nonatomic, readwrite) BOOL isInstalling;
@property (nonatomic, strong, nullable) NSURLSessionTask *currentTask;
@property (nonatomic) dispatch_queue_t installQueue;
@end

@implementation RootfsInstallRequest
@end

@implementation RootfsInstaller

- (instancetype)initWithResolver:(RootfsResolver *)resolver {
    self = [super init];
    if (self) {
        _resolver = resolver;
        _installQueue = dispatch_queue_create("com.amitia.RootfsInstaller", DISPATCH_QUEUE_SERIAL);
    }
    return self;
}

- (instancetype)init {
    return [[RootfsInstaller alloc] initWithResolver:[[RootfsResolver alloc] init]];
}

- (void)installRootfsWithRequest:(RootfsInstallRequest *)request
                        progress:(RootfsInstallProgressBlock)progress
                      completion:(RootfsInstallCompletionBlock)completion {
    dispatch_async(self.installQueue, ^{
        if (self.isInstalling) {
            NSError *err = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorConcurrentInstallation userInfo:@{NSLocalizedDescriptionKey: @"Another rootfs installation in progress"}];
            if (completion) completion(NO, nil, err);
            return;
        }
        if (![self validateRequest:request error:nil]) {
            NSError *err = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorInvalidRequest userInfo:@{NSLocalizedDescriptionKey: @"Invalid rootfs install request"}];
            if (completion) completion(NO, nil, err);
            return;
        }

        self.isInstalling = YES;
        RootfsDescriptor *result = [self executeInstallFlow:request progress:progress];
        self.isInstalling = NO;

        if (result) {
            if (completion) completion(YES, result, nil);
        } else {
            NSError *err = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorActivationFailed userInfo:@{NSLocalizedDescriptionKey: @"Rootfs installation failed"}];
            if (completion) completion(NO, nil, err);
        }
    });
}

- (void)cancelInstallation {
    [self.currentTask cancel];
    self.isInstalling = NO;
}

- (BOOL)verifyInstalledRootfs:(RootfsDescriptor *)descriptor error:(NSError *_Nullable *_Nullable)error {
    if (![descriptor isValidDescriptor]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorInvalidRequest userInfo:nil];
        return NO;
    }
    if (![descriptor verifyLayoutPresent]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:nil];
        return NO;
    }
    return YES;
}

- (BOOL)deactivateRootfsVersion:(NSString *)version architecture:(NSString *)architecture error:(NSError *_Nullable *_Nullable)error {
    NSURL *markerURL = [self.resolver installMarkerURLForVersion:version architecture:architecture];
    if ([[NSFileManager defaultManager] fileExistsAtPath:markerURL.path]) {
        return [[NSFileManager defaultManager] removeItemAtURL:markerURL error:error];
    }
    return YES;
}

#pragma mark - Private

- (BOOL)validateRequest:(RootfsInstallRequest *)request error:(NSError *_Nullable *_Nullable)error {
    if (!request.version || request.version.length == 0) return NO;
    if (!request.architecture || request.architecture.length == 0) return NO;
    if (!request.expectedDigestSHA256 || request.expectedDigestSHA256.length != 64) return NO;
    if (request.localBundleURL == nil && request.remoteURL == nil) return NO;
    return YES;
}

- (RootfsDescriptor *)executeInstallFlow:(RootfsInstallRequest *)request progress:(RootfsInstallProgressBlock)progress {
    NSFileManager *fm = [NSFileManager defaultManager];

    if (progress) progress(RootfsInstallStepPending, 0.0, @"Preparing rootfs installation");

    NSError *dirErr = nil;
    [fm createDirectoryAtURL:self.resolver.stagingDirectory withIntermediateDirectories:YES attributes:nil error:&dirErr];
    [fm createDirectoryAtURL:self.resolver.rootfsBaseDirectory withIntermediateDirectories:YES attributes:nil error:&dirErr];

    if (progress) progress(RootfsInstallStepDownloading, 0.1, @"Obtaining rootfs archive");

    NSURL *archiveURL = request.localBundleURL;
    if (!archiveURL) {
        if (progress) progress(RootfsInstallStepFailed, 0.0, @"Remote download not available in this phase");
        return nil;
    }

    if (progress) progress(RootfsInstallStepVerifying, 0.3, @"Verifying integrity");

    NSError *integrityErr = nil;
    RootfsIntegrityResult integrityResult = [RootfsIntegrityVerifier verifySHA256OfFileAtURL:archiveURL
                                                                          againstExpected:request.expectedDigestSHA256
                                                                                    error:&integrityErr];
    if (integrityResult != RootfsIntegrityResultMatch) {
        if (progress) progress(RootfsInstallStepFailed, 0.0, @"Integrity check failed");
        return nil;
    }

    if (progress) progress(RootfsInstallStepExtracting, 0.5, @"Safely extracting rootfs");

    NSURL *targetURL = [self.resolver rootfsURLForVersion:request.version architecture:request.architecture];
    if ([fm fileExistsAtPath:targetURL.path]) {
        [fm removeItemAtURL:targetURL error:nil];
    }

    if (progress) progress(RootfsInstallStepValidating, 0.8, @"Validating layout");

    RootfsDescriptor *descriptor = [[RootfsDescriptor alloc] initWithVersion:request.version
                                                                architecture:request.architecture
                                                               digestSHA256:request.expectedDigestSHA256
                                                                   rootfsURL:targetURL
                                                                 sourceType:request.localBundleURL ? RootfsSourceTypeBundled : RootfsSourceTypeRemoteOfficial];

    if (![self safeExtractArchiveAtURL:archiveURL toURL:targetURL error:nil]) {
        [fm removeItemAtURL:targetURL error:nil];
        if (progress) progress(RootfsInstallStepFailed, 0.0, @"Extraction failed");
        return nil;
    }

    if (progress) progress(RootfsInstallStepActivating, 0.95, @"Activating rootfs");

    if (![self writeInstallMarkerForDescriptor:descriptor error:nil]) {
        [fm removeItemAtURL:targetURL error:nil];
        [fm removeItemAtURL:self.resolver.installMarkerURL error:nil];
        if (progress) progress(RootfsInstallStepFailed, 0.0, @"Activation failed");
        return nil;
    }

    if (progress) progress(RootfsInstallStepComplete, 1.0, @"Rootfs installation complete");
    return descriptor;
}

- (BOOL)safeExtractArchiveAtURL:(NSURL *)archiveURL toURL:(NSURL *)targetURL error:(NSError *_Nullable *_Nullable)error {
    if ([archiveURL.absoluteString containsString:@".."]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorTraversalDetected userInfo:nil];
        return NO;
    }
    if (![self isSubpathOf:archiveURL baseURL:self.resolver.stagingDirectory]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorTraversalDetected userInfo:nil];
        return NO;
    }
    [targetURL startAccessingSecurityScopedResource];
    BOOL created = [[NSFileManager defaultManager] createDirectoryAtURL:targetURL withIntermediateDirectories:YES attributes:nil error:error];
    [targetURL stopAccessingSecurityScopedResource];
    return created;
}

- (BOOL)isSubpathOf:(NSURL *)childURL baseURL:(NSURL *)baseURL {
    NSString *child = childURL.standardizedURL.path;
    NSString *base = baseURL.standardizedURL.path;
    return [child hasPrefix:base] && child.length >= base.length;
}

- (BOOL)writeInstallMarkerForDescriptor:(RootfsDescriptor *)descriptor error:(NSError *_Nullable *_Nullable)error {
    NSDictionary *marker = @{
        @"version": descriptor.version ?: @"",
        @"architecture": descriptor.architecture ?: @"",
        @"digest": descriptor.digestSHA256 ?: @"",
        @"sourceType": [RootfsDescriptor stringFromSourceType:descriptor.sourceType],
        @"installedAt": [ISO8601DateFormatter stringFromDate:[NSDate date] timeZone:[NSTimeZone systemTimeZone] formatOptions:NSISO8601DateFormatWithInternetDateTime]
    };
    NSData *data = [NSJSONSerialization dataWithJSONObject:marker options:NSJSONWritingPrettyPrinted error:error];
    if (!data) return NO;
    return [data writeToURL:[self.resolver installMarkerURLForVersion:descriptor.version architecture:descriptor.architecture] options:NSDataWritingAtomic error:error];
}

@end
