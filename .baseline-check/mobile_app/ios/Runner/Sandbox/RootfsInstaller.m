#import "RootfsInstaller.h"
#import "RootfsIntegrityVerifier.h"
#import "RootfsArchiveExtractor.h"
#import <sys/statvfs.h>
#import <sys/errno.h>

NSErrorDomain const RootfsInstallerErrorDomain = @"com.amitia.RootfsInstaller";

static NSString * const kActiveManifestName = @"active.manifest";
static NSString * const kVersionManifestName = @"rootfs.manifest.json";
static const int64_t kMinFreeSpaceBytes = 1024LL * 1024 * 1024;
static NSString * const kCanonicalFormat = @"ish_fakefs";
static NSString * const kCanonicalArch = @"aarch64";
static NSString * const kVersionSchemaVersion = @"1";

static NSCharacterSet *kSafeRootfsChars = nil;

@implementation RootfsInstallResult

- (instancetype)initWithDescriptor:(RootfsDescriptor *)descriptor activated:(BOOL)activated requiresRestart:(BOOL)restart {
    self = [super init];
    if (self) {
        _descriptor = descriptor;
        _activated = activated;
        _requiresSandboxRestart = restart;
    }
    return self;
}

@end

@implementation RootfsInstallRequest

- (instancetype)init {
    self = [super init];
    if (self) {
        _allowCellularDownload = NO;
        _forceReplace = NO;
        _maxArchiveBytes = 512LL * 1024 * 1024;
        _architecture = kCanonicalArch;
    }
    return self;
}

@end

@interface RootfsInstaller ()
@property (nonatomic, strong, readwrite) RootfsResolver *resolver;
@property (nonatomic, readwrite) BOOL isInstalling;
@property (nonatomic, copy, readwrite, nullable) NSString *currentInstallID;
@property (nonatomic, readwrite) BOOL cancellationRequested;
@property (nonatomic, strong, nullable) NSURLSessionTask *currentTask;
@property (nonatomic, strong) dispatch_queue_t installQueue;
@property (nonatomic, strong) RootfsArchiveExtractor *extractor;
@end

@implementation RootfsInstaller

+ (void)initialize {
    if (self == [RootfsInstaller class]) {
        kSafeRootfsChars = [NSCharacterSet characterSetWithCharactersInString:@"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"];
    }
}

- (instancetype)initWithResolver:(RootfsResolver *)resolver {
    self = [super init];
    if (self) {
        _resolver = resolver;
        _installQueue = dispatch_queue_create("com.amitia.RootfsInstaller", DISPATCH_QUEUE_SERIAL);
        _extractor = [RootfsArchiveExtractor defaultExtractor];
    }
    return self;
}

- (instancetype)init {
    return [[RootfsInstaller alloc] initWithResolver:[[RootfsResolver alloc] init]];
}

- (void)installRootfsWithRequest:(RootfsInstallRequest *)request
                        progress:(RootfsInstallProgressBlock)progress
                      completion:(void(^)(BOOL success, RootfsInstallResult *_Nullable, NSError *_Nullable))completion {
    dispatch_async(self.installQueue, ^{
        if (self.isInstalling) {
            NSError *err = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorConcurrentInstallation userInfo:@{NSLocalizedDescriptionKey: @"Another rootfs installation in progress"}];
            dispatch_async(dispatch_get_main_queue(), ^{ if (completion) completion(NO, nil, err); });
            return;
        }

        NSError *validateErr = nil;
        if (![self validateRequest:request error:&validateErr]) {
            dispatch_async(dispatch_get_main_queue(), ^{ if (completion) completion(NO, nil, validateErr); });
            return;
        }

        self.isInstalling = YES;
        self.cancellationRequested = NO;
        self.currentInstallID = [[NSUUID UUID] UUIDString];

        NSError *flowErr = nil;
        RootfsInstallResult *result = [self executeInstallFlow:request progress:progress error:&flowErr];

        self.isInstalling = NO;
        self.cancellationRequested = NO;
        self.currentInstallID = nil;

        if (result) {
            dispatch_async(dispatch_get_main_queue(), ^{ if (completion) completion(YES, result, nil); });
        } else {
            dispatch_async(dispatch_get_main_queue(), ^{ if (completion) completion(NO, nil, flowErr); });
        }
    });
}

- (void)cancelInstallation {
    self.cancellationRequested = YES;
    [self.currentTask cancel];
}

- (BOOL)verifyInstalledRootfs:(RootfsDescriptor *)descriptor error:(NSError **)error {
    if (!descriptor || ![descriptor isValidDescriptor]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorInvalidRequest userInfo:nil];
        return NO;
    }
    if (descriptor.format == RootfsFormatISHFakeFS) {
        if (![descriptor verifyISHFakeFSLayout]) {
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:nil];
            return NO;
        }
    } else if (![descriptor verifyLayoutPresent]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:nil];
        return NO;
    }
    return YES;
}

- (BOOL)deactivateRootfsVersion:(NSString *)version architecture:(NSString *)architecture error:(NSError **)error {
    RootfsDescriptor *active = [self.resolver resolveCurrentRootfs];
    if (!active) return YES;

    if ([active.version isEqualToString:version] && [active.architecture isEqualToString:architecture]) {
        NSData *empty = [NSData data];
        BOOL ok = [empty writeToURL:self.resolver.activeManifestURL options:NSDataWritingAtomic error:error];
        return ok;
    }
    return YES;
}

- (BOOL)isRootfsActive {
    RootfsDescriptor *d = [self.resolver resolveCurrentRootfs];
    return d != nil;
}

- (BOOL)checkCancelled:(NSError **)error {
    if (self.cancellationRequested) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorCancelled userInfo:nil];
        return YES;
    }
    return NO;
}

- (BOOL)validateRequest:(RootfsInstallRequest *)request error:(NSError **)error {
    if (!request.version || request.version.length == 0) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorInvalidRequest userInfo:@{NSLocalizedDescriptionKey: @"version required"}];
        return NO;
    }
    if ([request.version rangeOfCharacterFromSet:[kSafeRootfsChars invertedSet]].location != NSNotFound) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorInvalidRequest userInfo:@{NSLocalizedDescriptionKey: @"version contains unsafe characters"}];
        return NO;
    }

    NSString *arch = request.architecture ?: kCanonicalArch;
    if (![arch isEqualToString:kCanonicalArch]) {
        arch = kCanonicalArch;
        request.architecture = arch;
    }

    if (![RootfsIntegrityVerifier isValidSHA256Digest:request.expectedDigestSHA256]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorDigestMalformed userInfo:@{NSLocalizedDescriptionKey: @"digest must be 64 hex chars lowercase"}];
        return NO;
    }

    if (request.localBundleURL == nil && request.remoteURL == nil) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSourceUnavailable userInfo:nil];
        return NO;
    }

    if (request.localBundleURL != nil) {
        BOOL isDir = NO;
        if (![[NSFileManager defaultManager] fileExistsAtPath:request.localBundleURL.path isDirectory:&isDir] || isDir) {
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSourceUnavailable userInfo:nil];
            return NO;
        }
    }

    return YES;
}

- (RootfsInstallResult *)executeInstallFlow:(RootfsInstallRequest *)request
                                   progress:(RootfsInstallProgressBlock)progress
                                      error:(NSError **)error {
    if ([self checkCancelled:error]) return nil;
    if (progress) progress(RootfsInstallStepValidating, 0.02, @"Validating request");

    NSString *arch = request.architecture ?: kCanonicalArch;
    NSString *version = request.version;

    if (progress) progress(RootfsInstallStepResolvingSource, 0.05, @"Resolving source");

    NSURL *sourceURL = request.localBundleURL;
    NSString *stagingPath = [self.resolver.stagingDirectory.path stringByAppendingPathComponent:self.currentInstallID];
    NSURL *stagingDir = [NSURL fileURLWithPath:stagingPath isDirectory:YES];
    [[NSFileManager defaultManager] createDirectoryAtURL:stagingDir withIntermediateDirectories:YES attributes:nil error:nil];

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (request.remoteURL != nil) {
        if (![request.remoteURL.scheme isEqualToString:@"https"]) {
            [self cleanupStaging:stagingDir];
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorRemoteRedirectRejected userInfo:nil];
            return nil;
        }

        if (progress) progress(RootfsInstallStepDownloading, 0.10, @"Downloading rootfs");

        NSError *dlErr = nil;
        sourceURL = [self downloadArchive:request.remoteURL maxSize:request.maxArchiveBytes error:&dlErr];
        if (!sourceURL) {
            [self cleanupStaging:stagingDir];
            if (error) *error = dlErr ?: [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSourceUnavailable userInfo:nil];
            return nil;
        }
    }

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (progress) progress(RootfsInstallStepVerifying, 0.30, @"Verifying archive integrity");

    NSError *integrityErr = nil;
    RootfsIntegrityResult integrityResult = [RootfsIntegrityVerifier verifySHA256OfFileAtURL:sourceURL againstExpected:request.expectedDigestSHA256 error:&integrityErr];
    if (integrityResult == RootfsIntegrityResultDigestMalformed) {
        [self cleanupStaging:stagingDir];
        if (error) *error = integrityErr;
        return nil;
    }
    if (integrityResult != RootfsIntegrityResultMatch) {
        [self cleanupStaging:stagingDir];
        RootfsInstallerError code = (integrityResult == RootfsIntegrityResultFileMissing) ? RootfsInstallerErrorSourceUnavailable : RootfsInstallerErrorIntegrityMismatch;
        NSString *msg = (integrityResult == RootfsIntegrityResultFileMissing) ? @"Archive file not found" : @"Archive integrity check failed";
        if (error) *error = integrityErr ?: [NSError errorWithDomain:RootfsInstallerErrorDomain code:code userInfo:@{NSLocalizedDescriptionKey: msg}];
        return nil;
    }

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (progress) progress(RootfsInstallStepExtracting, 0.40, @"Safely extracting rootfs");

    self.extractor.cancellationRequested = NO;
    NSError *extractErr = nil;
    if (![self.extractor extractArchiveAtURL:sourceURL toDirectory:stagingDir error:&extractErr]) {
        [self cleanupStaging:stagingDir];
        RootfsInstallerError code = RootfsInstallerErrorExtractionFailed;
        if (extractErr.code == RootfsExtractionErrorPathTraversal) code = RootfsInstallerErrorTraversalDetected;
        else if (extractErr.code == RootfsExtractionErrorSymlinkEscape) code = RootfsInstallerErrorSymlinkEscapeDetected;
        else if (extractErr.code == RootfsExtractionErrorTooManyEntries) code = RootfsInstallerErrorTooManyEntries;
        else if (extractErr.code == RootfsExtractionErrorSizeExceeded) code = RootfsInstallerErrorExtractedSizeExceeded;
        else if (extractErr.code == RootfsExtractionErrorCancelled) code = RootfsInstallerErrorCancelled;
        if (error) *error = extractErr ?: [NSError errorWithDomain:RootfsInstallerErrorDomain code:code userInfo:nil];
        return nil;
    }

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (progress) progress(RootfsInstallStepValidatingPackage, 0.60, @"Validating rootfs package");

    NSError *pkgErr = nil;
    RootfsDescriptor *stagedDesc = [self validateStagedPackageAtURL:stagingDir version:version architecture:arch error:&pkgErr];
    if (!stagedDesc) {
        [self cleanupStaging:stagingDir];
        if (error) *error = pkgErr;
        return nil;
    }

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (progress) progress(RootfsInstallStepPreparingTarget, 0.75, @"Preparing target directory");

    NSURL *targetURL = [self.resolver.rootfsURLForVersion:version architecture:arch];
    if (targetURL && [[NSFileManager defaultManager] fileExistsAtPath:targetURL.path]) {
        RootfsDescriptor *existing = [self.resolver resolveInstalledRootfsVersion:version architecture:arch];
        if (existing && existing.packageDigestSHA256 && stagedDesc.packageDigestSHA256 && [existing.packageDigestSHA256 isEqualToString:stagedDesc.packageDigestSHA256]) {
            [self cleanupStaging:stagingDir];
            BOOL activated = [self isVersionActive:version architecture:arch];
            if (!activated) {
                NSError *actErr = nil;
                if (![self writeActiveManifestVersion:version architecture:arch error:&actErr]) {
                    if (error) *error = actErr;
                    return nil;
                }
                activated = YES;
            }
            return [[RootfsInstallResult alloc] initWithDescriptor:existing activated:activated requiresRestart:NO];
        }
        if (!request.forceReplace) {
            [self cleanupStaging:stagingDir];
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorVersionConflict userInfo:@{NSLocalizedDescriptionKey: @"Version already exists with different digest. Use forceReplace."}];
            return nil;
        }
        [[NSFileManager defaultManager] removeItemAtURL:targetURL error:nil];
    }

    if (progress) progress(RootfsInstallStepAtomicMove, 0.85, @"Moving to final location");

    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *moveErr = nil;
    if (![fm moveItemAtURL:stagingDir toURL:targetURL error:&moveErr]) {
        NSError *copyErr = nil;
        if (![fm copyItemAtURL:stagingDir toURL:targetURL error:&copyErr]) {
            [self cleanupStaging:stagingDir];
            if (fm.fileExistsAtPath(targetURL.path)) [fm removeItemAtURL:targetURL error:nil];
            if (error) *error = copyErr ?: moveErr;
            return nil;
        }
    }

    if ([self checkCancelled:error]) { [self cleanupStaging:stagingDir]; return nil; }

    if (progress) progress(RootfsInstallStepWritingManifest, 0.90, @"Writing version manifest");

    NSError *manifestErr = nil;
    if (![self writeVersionManifestForDescriptor:stagedDesc error:&manifestErr]) {
        [fm removeItemAtURL:targetURL error:nil];
        [self cleanupStaging:stagingDir];
        if (error) *error = manifestErr;
        return nil;
    }

    if ([self checkCancelled:error]) { return nil; }

    if (progress) progress(RootfsInstallStepAtomicActivation, 0.95, @"Activating rootfs");

    NSError *actErr = nil;
    if (![self writeActiveManifestVersion:version architecture:arch error:&actErr]) {
        [fm removeItemAtURL:targetURL error:nil];
        [self cleanupStaging:stagingDir];
        if (error) *error = actErr;
        return nil;
    }

    if (progress) progress(RootfsInstallStepCleanup, 0.98, @"Cleaning up");
    [self cleanupStaging:stagingDir];

    if (progress) progress(RootfsInstallStepComplete, 1.0, @"Rootfs installation complete");

    RootfsDescriptor *finalDesc = [self.resolver resolveInstalledRootfsVersion:version architecture:arch];
    if (!finalDesc) finalDesc = stagedDesc;

    return [[RootfsInstallResult alloc] initWithDescriptor:finalDesc activated:YES requiresRestart:YES];
}

- (RootfsDescriptor *)validateStagedPackageAtURL:(NSURL *)stagingDir version:(NSString *)version architecture:(NSString *)arch error:(NSError **)error {
    NSString *dataPath = [stagingDir.path stringByAppendingPathComponent:@"data"];
    NSString *metaDbPath = [stagingDir.path stringByAppendingPathComponent:@"meta.db"];
    NSString *manifestPath = [stagingDir.path stringByAppendingPathComponent:kVersionManifestName];

    NSFileManager *fm = [NSFileManager defaultManager];
    BOOL isDir = NO;

    if (![fm fileExistsAtPath:dataPath isDirectory:&isDir] || !isDir) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:@{NSLocalizedDescriptionKey: @"Missing data/ in package"}];
        return nil;
    }
    if (![fm fileExistsAtPath:metaDbPath isDirectory:&isDir] || isDir) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:@{NSLocalizedDescriptionKey: @"Missing or invalid meta.db"}];
        return nil;
    }

    NSString *realMetaDb = [fm destinationOfSymbolicLinkAtPath:metaDbPath error:nil] ?: metaDbPath;
    if (![fm fileExistsAtPath:realMetaDb]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSymlinkEscapeDetected userInfo:nil];
        return nil;
    }

    NSString *stagingRoot = stagingDir.URLByResolvingSymlinksInPath.path;
    if (![realMetaDb hasPrefix:stagingRoot]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSymlinkEscapeDetected userInfo:nil];
        return nil;
    }

    NSString *binShPath = [dataPath stringByAppendingPathComponent:@"bin/sh"];
    if (![fm fileExistsAtPath:binShPath]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:@{NSLocalizedDescriptionKey: @"Missing data/bin/sh"}];
        return nil;
    }

    NSString *etcPath = [dataPath stringByAppendingPathComponent:@"etc"];
    if (![fm fileExistsAtPath:etcPath isDirectory:&isDir] || !isDir) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:nil];
        return nil;
    }

    NSString *usrPath = [dataPath stringByAppendingPathComponent:@"usr"];
    if (![fm fileExistsAtPath:usrPath isDirectory:&isDir] || !isDir) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:nil];
        return nil;
    }

    NSString *realDataPath = [fm destinationOfSymbolicLinkAtPath:dataPath error:nil];
    if (realDataPath) {
        if (![fm fileExistsAtPath:realDataPath isDirectory:&isDir] || !isDir) {
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSymlinkEscapeDetected userInfo:nil];
            return nil;
        }
        if (![realDataPath hasPrefix:stagingRoot]) {
            if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSymlinkEscapeDetected userInfo:nil];
            return nil;
        }
    }

    NSNumber *metaSize = nil;
    NSError *resErr = nil;
    if (![stagingDir getResourceValue:&metaSize forKey:NSFileSizeKey error:&resErr] || metaSize.longLongValue <= 0) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorLayoutInvalid userInfo:@{NSLocalizedDescriptionKey: @"meta.db invalid"}];
        return nil;
    }

    NSString *manifestDigest = nil;
    if ([fm fileExistsAtPath:manifestPath]) {
        NSError *readErr = nil;
        NSData *mfData = [NSData dataWithContentsOfURL:[NSURL fileURLWithPath:manifestPath] options:0 error:&readErr];
        if (mfData && mfData.length > 0) {
            NSDictionary *mf = [NSJSONSerialization JSONObjectWithData:mfData options:0 error:nil];
            if (mf && [mf[@"version"] isEqualToString:version] && [mf[@"architecture"] isEqualToString:arch]) {
                manifestDigest = mf[@"packageSha256"];
            }
        }
    }

    NSString *packageDigest = manifestDigest ?: @"";
    return [[RootfsDescriptor alloc] initWithVersion:version
                                       architecture:arch
                                      digestSHA256:packageDigest
                                          rootfsURL:stagingDir
                                           mountURL:[NSURL fileURLWithPath:dataPath isDirectory:YES]
                                         sourceType:RootfsSourceTypeBundled
                                             format:RootfsFormatISHFakeFS
                                 packageDigestSHA256:packageDigest
                                      formatVersion:kVersionSchemaVersion
                                       manifestPath:manifestPath
                                              state:RootfsStateInstalled];
}

- (BOOL)writeVersionManifestForDescriptor:(RootfsDescriptor *)descriptor error:(NSError **)error {
    NSDictionary *manifest = @{
        @"schemaVersion": @1,
        @"format": [RootfsDescriptor stringFromFormat:descriptor.format],
        @"formatVersion": descriptor.formatVersion ?: kVersionSchemaVersion,
        @"distribution": @"alpine",
        @"version": descriptor.version,
        @"architecture": descriptor.architecture,
        @"packageSha256": descriptor.packageDigestSHA256 ?: descriptor.digestSHA256,
        @"sourceType": [RootfsDescriptor stringFromSourceType:descriptor.sourceType],
        @"installedAt": [[ISO8601DateFormatter new] stringFromDate:[NSDate date]]
    };

    NSError *serErr = nil;
    NSData *data = [NSJSONSerialization dataWithJSONObject:manifest options:0 error:&serErr];
    if (!data) {
        if (error) *error = serErr ?: [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorManifestInvalid userInfo:nil];
        return NO;
    }

    NSString *tempPath = [NSString stringWithFormat:@"%@.%@", descriptor.manifestPath, self.currentInstallID];
    NSError *writeErr = nil;
    if (![data writeToFile:tempPath options:NSDataWritingAtomic error:&writeErr]) {
        if (error) *error = writeErr;
        return NO;
    }

    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *replaceErr = nil;
    if (![fm replaceItemAtURL:[NSURL fileURLWithPath:descriptor.manifestPath]
               withItemAtURL:[NSURL fileURLWithPath:tempPath]
              backupItemName:nil
                     options:0
            resultingItemURL:nil
                       error:&replaceErr]) {
        [fm removeItemAtPath:tempPath error:nil];
        if (error) *error = replaceErr;
        return NO;
    }
    return YES;
}

- (BOOL)writeActiveManifestVersion:(NSString *)version architecture:(NSString *)arch error:(NSError **)error {
    NSDictionary *active = @{
        @"schemaVersion": @1,
        @"version": version,
        @"architecture": arch
    };

    NSData *data = [NSJSONSerialization dataWithJSONObject:active options:0 error:error];
    if (!data) return NO;

    NSString *tempPath = [self.resolver.activeManifestURL.path stringByAppendingPathExtension:self.currentInstallID];
    NSError *writeErr = nil;
    if (![data writeToFile:tempPath options:NSDataWritingAtomic error:&writeErr]) {
        if (error) *error = writeErr;
        return NO;
    }

    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *replaceErr = nil;
    if (![fm replaceItemAtURL:self.resolver.activeManifestURL
               withItemAtURL:[NSURL fileURLWithPath:tempPath]
              backupItemName:nil
                     options:0
            resultingItemURL:nil
                       error:&replaceErr]) {
        [fm removeItemAtPath:tempPath error:nil];
        if (error) *error = replaceErr;
        return NO;
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSNotificationCenter defaultCenter] postNotificationName:kRootfsActiveVersionDidChangeNotification
                                                            object:nil
                                                          userInfo:@{@"version": version, @"architecture": arch}];
    });

    return YES;
}

- (BOOL)isVersionActive:(NSString *)version architecture:(NSString *)arch {
    RootfsDescriptor *current = [self.resolver resolveCurrentRootfs];
    return current && [current.version isEqualToString:version] && [current.architecture isEqualToString:arch];
}

- (NSURL *)downloadArchive:(NSURL *)remoteURL maxSize:(int64_t)maxSize error:(NSError **)error {
    if (![remoteURL.scheme isEqualToString:@"https"]) {
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorRemoteRedirectRejected userInfo:nil];
        return nil;
    }

    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    __block NSURL *resultURL = nil;
    __block NSError *resultError = nil;
    __block int64_t downloaded = 0;

    NSString *tempName = [NSString stringWithFormat:@"download-%@.zip", self.currentInstallID];
    NSURL *tempURL = [NSURL fileURLWithPath:[NSTemporaryDirectory() stringByAppendingPathComponent:tempName]];

    NSURLSessionConfiguration *cfg = [NSURLSessionConfiguration ephemeralSessionConfiguration];
    cfg.allowsCellularAccess = NO;
    cfg.requestCachePolicy = NSURLRequestReloadIgnoringLocalCacheData;
    NSURLSession *session = [NSURLSession sessionWithConfiguration:cfg delegate:(id<NSURLSessionDelegate>)self delegateQueue:nil];

    NSURLSessionDownloadTask *task = [session downloadTaskWithURL:remoteURL completionHandler:^(NSURL *loc,NSURLResponse *resp,NSError *err) {
        if (err || !loc) {
            resultError = err ?: [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorSourceUnavailable userInfo:nil];
            dispatch_semaphore_signal(sem);
            return;
        }
        NSData *data = [NSData dataWithContentsOfURL:loc options:0 error:nil];
        int64_t sz = data ? (int64_t)data.length : 0;
        if (maxSize > 0 && sz > maxSize) {
            resultError = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorArchiveTooLarge userInfo:nil];
            dispatch_semaphore_signal(sem);
            return;
        }
        [data writeToURL:tempURL options:NSDataWritingAtomic error:nil];
        resultURL = tempURL;
        dispatch_semaphore_signal(sem);
    }];
    self.currentTask = task;
    [task resume];
    dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
    [session invalidateAndCancel];
    self.currentTask = nil;

    if (resultError) {
        [[NSFileManager defaultManager] removeItemAtURL:tempURL error:nil];
        if (error) *error = resultError;
        return nil;
    }

    int64_t finalSize = [[NSFileManager defaultManager] attributesOfItemAtPath:tempURL.path error:nil].fileSize;
    if (maxSize > 0 && finalSize > maxSize) {
        [[NSFileManager defaultManager] removeItemAtURL:tempURL error:nil];
        if (error) *error = [NSError errorWithDomain:RootfsInstallerErrorDomain code:RootfsInstallerErrorArchiveTooLarge userInfo:nil];
        return nil;
    }

    return resultURL;
}

- (void)cleanupStaging:(NSURL *)stagingDir {
    if (!stagingDir) return;
    [[NSFileManager defaultManager] removeItemAtURL:stagingDir error:nil];
}

- (void)URLSession:(NSURLSession *)session task:(NSURLSessionTask *)task willPerformHTTPRedirection:(NSHTTPURLResponse *)response newRequest:(NSURLRequest *)request completionHandler:(void (^)(NSURLRequest * _Nullable))completionHandler {
    if (![request.URL.scheme isEqualToString:@"https"]) {
        completionHandler(nil);
        return;
    }
    completionHandler(request);
}

@end
