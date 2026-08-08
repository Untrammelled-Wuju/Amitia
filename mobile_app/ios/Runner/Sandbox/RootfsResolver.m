#import "RootfsResolver.h"

static NSString * const kRootfsDirectoryName = @"runtime/rootfs";
static NSString * const kVersionsSubDirectory = @"versions";
static NSString * const kActiveManifestName = @"active.manifest";
static NSString * const kVersionManifestName = @"rootfs.manifest.json";
static NSString * const kStagingSubDirectory = @"amitia/rootfs-staging";

@implementation RootfsResolver

- (instancetype)initWithBaseDirectory:(NSURL *)baseDirectory {
    self = [super init];
    if (self) {
        _rootfsBaseDirectory = baseDirectory;
        _versionsDirectory = [baseDirectory URLByAppendingPathComponent:kVersionsSubDirectory isDirectory:YES];
        _activeManifestURL = [baseDirectory URLByAppendingPathComponent:kActiveManifestName isDirectory:NO];
        _stagingDirectory = [RootfsResolver defaultStagingDirectory];
    }
    return self;
}

- (instancetype)init {
    return [self initWithBaseDirectory:[RootfsResolver defaultRootfsBaseDirectory]];
}

- (NSURL *)rootfsURLForVersion:(NSString *)version architecture:(NSString *)architecture {
    NSString *name = [NSString stringWithFormat:@"alpine-%@-%@", version, architecture];
    return [self.versionsDirectory URLByAppendingPathComponent:name isDirectory:YES];
}

- (NSString *)manifestPathForVersion:(NSString *)version architecture:(NSString *)architecture {
    return [[self rootfsURLForVersion:version architecture:architecture].path stringByAppendingPathComponent:kVersionManifestName];
}

- (RootfsDescriptor *)resolveCurrentRootfs {
    NSError *err = nil;
    NSData *data = [NSData dataWithContentsOfURL:self.activeManifestURL options:0 error:&err];
    if (!data || data.length == 0) return nil;

    NSDictionary *json = [NSJSONSerialization JSONObjectWithData:data options:0 error:&err];
    if (!json || ![json isKindOfClass:[NSDictionary class]]) return nil;

    NSNumber *schemaVersion = json[@"schemaVersion"];
    if (!schemaVersion || schemaVersion.integerValue != 1) return nil;

    NSString *version = json[@"version"];
    NSString *arch = json[@"architecture"];
    if (!version || version.length == 0 || !arch || arch.length == 0) return nil;

    return [self resolveInstalledRootfsVersion:version architecture:arch];
}

- (RootfsDescriptor *)resolveInstalledRootfsVersion:(NSString *)version architecture:(NSString *)architecture {
    NSString *manifestPath = [self manifestPathForVersion:version architecture:architecture];
    NSFileManager *fm = [NSFileManager defaultManager];
    BOOL isDir = NO;
    if ([fm fileExistsAtPath:manifestPath isDirectory:&isDir] || isDir) return nil;

    NSError *err = nil;
    NSData *data = [NSData dataWithContentsOfURL:[NSURL fileURLWithPath:manifestPath] options:0 error:&err];
    if (!data || data.length == 0) return nil;

    NSDictionary *json = [NSJSONSerialization JSONObjectWithData:data options:0 error:&err];
    if (!json || ![json isKindOfClass:[NSDictionary class]]) return nil;

    NSNumber *schemaVersion = json[@"schemaVersion"];
    if (!schemaVersion || schemaVersion.integerValue != 1) return nil;

    NSString *formatStr = json[@"format"];
    NSString *fmtVersion = json[@"formatVersion"];
    NSString *distVersion = json[@"version"];
    NSString *arch = json[@"architecture"];
    NSString *pkgDigest = json[@"packageSha256"];
    NSString *sourceStr = json[@"sourceType"];

    if (![distVersion isEqualToString:version] || ![arch isEqualToString:architecture]) return nil;
    if (!formatStr || [formatStr isEqualToString:@"unknown"]) return nil;
    if (formatStr && [formatStr isEqualToString:@"ish_fakefs"] && (!pkgDigest || pkgDigest.length != 64)) return nil;

    NSURL *rootfsURL = [self rootfsURLForVersion:version architecture:architecture];
    if (![fm fileExistsAtPath:rootfsURL.path isDirectory:&isDir] || !isDir) return nil;

    RootfsFormat fmt = RootfsFormatUnknown;
    if ([formatStr isEqualToString:@"ish_fakefs"]) fmt = RootfsFormatISHFakeFS;

    NSURL *mountURL = [rootfsURL URLByAppendingPathComponent:@"data" isDirectory:YES];

    if (fmt == RootfsFormatISHFakeFS && ![self verifyFakeFSStructureAtURL:rootfsURL mountURL:mountURL]) {
        return [[RootfsDescriptor alloc] initWithVersion:version
                                            architecture:architecture
                                           digestSHA256:pkgDigest ?: @""
                                               rootfsURL:rootfsURL
                                                mountURL:nil
                                              sourceType:RootfsSourceTypeUnknown
                                                  format:fmt
                                      packageDigestSHA256:pkgDigest
                                           formatVersion:fmtVersion
                                            manifestPath:manifestPath
                                                   state:RootfsStateCorrupt];
    }

    RootfsSourceType srcType = [self sourceTypeFromString:sourceStr];

    return [[RootfsDescriptor alloc] initWithVersion:version
                                        architecture:architecture
                                       digestSHA256:pkgDigest ?: @""
                                           rootfsURL:rootfsURL
                                            mountURL:mountURL
                                          sourceType:srcType
                                              format:fmt
                                  packageDigestSHA256:pkgDigest
                                       formatVersion:fmtVersion
                                        manifestPath:manifestPath
                                               state:RootfsStateInstalled];
}

- (NSArray<RootfsDescriptor *> *)listInstalledRootfs {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *err = nil;
    NSArray<NSURL *>* entries = [fm contentsOfDirectoryAtURL:self.versionsDirectory
                                  includingPropertiesForKeys:@[NSURLIsDirectoryKey]
                                                     options:NSDirectoryEnumerationSkipsHiddenFiles
                                                       error:&err];
    if (!entries) return @[];

    NSMutableArray<RootfsDescriptor *> *results = [NSMutableArray array];
    for (NSURL *entry in entries) {
        NSNumber *isDir = nil;
        if (![entry getResourceValue:&isDir forKey:NSURLIsDirectoryKey error:nil] || !isDir.boolValue) continue;

        NSString *manifestPath = [entry.path stringByAppendingPathComponent:kVersionManifestName];
        if (![fm fileExistsAtPath:manifestPath isDirectory:nil]) continue;

        NSData *data = [NSData dataWithContentsOfURL:[NSURL fileURLWithPath:manifestPath] options:0 error:nil];
        if (!data) continue;

        NSDictionary *json = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
        if (!json || ![json isKindOfClass:[NSDictionary class]]) continue;

        NSString *version = json[@"version"];
        NSString *arch = json[@"architecture"];
        if (!version || !arch) continue;

        NSString *formatStr = json[@"format"];
        NSString *pkgDigest = json[@"packageSha256"];
        NSString *fmtVersion = json[@"formatVersion"];
        NSString *sourceStr = json[@"sourceType"];

        RootfsFormat fmt = [formatStr isEqualToString:@"ish_fakefs"] ? RootfsFormatISHFakeFS : RootfsFormatUnknown;
        RootfsSourceType srcType = [self sourceTypeFromString:sourceStr];

        NSURL *mountURL = [entry URLByAppendingPathComponent:@"data" isDirectory:YES];

        RootfsDescriptor *desc = [[RootfsDescriptor alloc] initWithVersion:version
                                                              architecture:arch
                                                             digestSHA256:pkgDigest ?: @""
                                                                 rootfsURL:entry
                                                                  mountURL:mountURL
                                                                sourceType:srcType
                                                                    format:fmt
                                                        packageDigestSHA256:pkgDigest
                                                             formatVersion:fmtVersion
                                                              manifestPath:manifestPath
                                                                     state:RootfsStateInstalled];
        [results addObject:desc];
    }

    [results sortUsingComparator:^NSComparisonResult(RootfsDescriptor *a, RootfsDescriptor *b) {
        NSComparisonResult r = [a.version compare:b.version options:NSNumericSearch];
        if (r != NSOrderedSame) return r;
        return [a.architecture compare:b.architecture];
    }];

    return [results copy];
}

- (BOOL)isInstalledVersion:(NSString *)version architecture:(NSString *)architecture {
    RootfsDescriptor *desc = [self resolveInstalledRootfsVersion:version architecture:architecture];
    if (!desc) return NO;
    return desc.state == RootfsStateInstalled;
}

- (BOOL)verifyFakeFSStructureAtURL:(NSURL *)rootfsURL mountURL:(NSURL *)mountURL {
    NSFileManager *fm = [NSFileManager defaultManager];
    BOOL isDir = NO;

    NSString *dataPath = mountURL.path;
    if (![fm fileExistsAtPath:dataPath isDirectory:&isDir] || !isDir) return NO;

    NSString *metaDbPath = [rootfsURL.path stringByAppendingPathComponent:@"meta.db"];
    if (![fm fileExistsAtPath:metaDbPath isDirectory:&isDir] || isDir) return NO;

    NSString *binShPath = [dataPath stringByAppendingPathComponent:@"bin/sh"];
    if (![fm fileExistsAtPath:binShPath]) return NO;

    NSError *err = nil;
    NSNumber *metaSize = nil;
    if (![rootfsURL getResourceValue:&metaSize forKey:NSFileSizeKey error:&err] || metaSize.longLongValue <= 0) return NO;

    return YES;
}

+ (NSURL *)defaultRootfsBaseDirectory {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSURL *base = [[fm URLsForDirectory:NSApplicationSupportDirectory inDomains:NSUserDomainMask].firstObject
                   URLByAppendingPathComponent:kRootfsDirectoryName isDirectory:YES];
    return base;
}

+ (NSURL *)defaultVersionsDirectory {
    return [[self defaultRootfsBaseDirectory] URLByAppendingPathComponent:kVersionsSubDirectory isDirectory:YES];
}

+ (NSURL *)defaultActiveManifestURL {
    return [[self defaultRootfsBaseDirectory] URLByAppendingPathComponent:kActiveManifestName isDirectory:NO];
}

+ (NSURL *)defaultStagingDirectory {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSURL *base = [[fm URLsForDirectory:NSCachesDirectory inDomains:NSUserDomainMask].firstObject
                   URLByAppendingPathComponent:kStagingSubDirectory isDirectory:YES];
    return base;
}

- (RootfsSourceType)sourceTypeFromString:(NSString *)str {
    if (!str) return RootfsSourceTypeUnknown;
    if ([str isEqualToString:@"bundled"]) return RootfsSourceTypeBundled;
    if ([str isEqualToString:@"remote_official"]) return RootfsSourceTypeRemoteOfficial;
    if ([str isEqualToString:@"user_imported"]) return RootfsSourceTypeUserImported;
    if ([str isEqualToString:@"legacy"]) return RootfsSourceTypeLegacy;
    return RootfsSourceTypeUnknown;
}

@end
