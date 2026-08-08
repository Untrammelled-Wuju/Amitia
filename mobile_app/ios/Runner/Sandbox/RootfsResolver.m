#import "RootfsResolver.h"

static NSString * const kRootfsDirectoryName = @"runtime/rootfs";
static NSString * const kStagingDirectoryName = @"tmp/rootfs_staging";
static NSString * const kInstallMarkerName = @"installed.manifest";

@implementation RootfsResolver

- (instancetype)initWithBaseDirectory:(NSURL *)baseDirectory {
    self = [super init];
    if (self) {
        _rootfsBaseDirectory = baseDirectory;
        _stagingDirectory = [baseDirectory URLByAppendingPathComponent:kStagingDirectoryName isDirectory:YES];
        _installMarkerURL = [baseDirectory URLByAppendingPathComponent:kInstallMarkerName isDirectory:NO];
    }
    return self;
}

- (instancetype)init {
    return [self initWithBaseDirectory:[RootfsResolver defaultRootfsBaseDirectory]];
}

- (NSURL *)rootfsURLForVersion:(NSString *)version architecture:(NSString *)architecture {
    NSString *name = [NSString stringWithFormat:@"alpine-%@-%@", version, architecture];
    return [self.rootfsBaseDirectory URLByAppendingPathComponent:name isDirectory:YES];
}

- (NSURL *)installMarkerURLForVersion:(NSString *)version architecture:(NSString *)architecture {
    return [[self rootfsURLForVersion:version architecture:architecture]
            URLByAppendingPathComponent:kInstallMarkerName isDirectory:NO];
}

- (RootfsDescriptor *)resolveCurrentRootfs {
    NSFileManager *fm = [NSFileManager defaultManager];
    BOOL isDir = NO;
    if (![fm fileExistsAtPath:self.installMarkerURL.path isDirectory:&isDir] || isDir) {
        return nil;
    }
    NSError *err = nil;
    NSString *content = [NSString stringWithContentsOfURL:self.installMarkerURL
                                                 encoding:NSUTF8StringEncoding error:&err];
    if (!content) return nil;
    NSData *data = [content dataUsingEncoding:NSUTF8StringEncoding];
    NSDictionary *json = [NSJSONSerialization JSONObjectWithData:data options:0 error:&err];
    if (!json) return nil;
    NSString *version = json[@"version"];
    NSString *arch = json[@"architecture"];
    NSString *digest = json[@"digest"];
    NSString *sourceStr = json[@"sourceType"];
    NSURL *rootfsURL = [self rootfsURLForVersion:version architecture:arch];
    if (![fm fileExistsAtPath:rootfsURL.path isDirectory:&isDir] || !isDir) {
        return nil;
    }
    RootfsDescriptor *desc = [[RootfsDescriptor alloc] initWithVersion:version
                                                          architecture:arch
                                                        digestSHA256:digest
                                                            rootfsURL:rootfsURL
                                                          sourceType:[self sourceTypeFromString:sourceStr]];
    return desc;
}

- (NSArray<RootfsDescriptor *> *)listInstalledRootfs {
    return @[];
}

- (BOOL)isInstalledVersion:(NSString *)version architecture:(NSString *)architecture {
    return [[NSFileManager defaultManager] fileExistsAtPath:[self installMarkerURLForVersion:version architecture:architecture].path];
}

+ (NSURL *)defaultRootfsBaseDirectory {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSURL *base = [[fm URLsForDirectory:NSApplicationSupportDirectory inDomains:NSUserDomainMask].firstObject
                   URLByAppendingPathComponent:kRootfsDirectoryName isDirectory:YES];
    return base;
}

+ (NSURL *)defaultStagingDirectory {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSURL *base = [[fm URLsForDirectory:NSCachesDirectory inDomains:NSUserDomainMask].firstObject
                   URLByAppendingPathComponent:kStagingDirectoryName isDirectory:YES];
    return base;
}

+ (NSURL *)defaultInstallMarkerURL {
    return [[self defaultRootfsBaseDirectory] URLByAppendingPathComponent:kInstallMarkerName isDirectory:NO];
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
