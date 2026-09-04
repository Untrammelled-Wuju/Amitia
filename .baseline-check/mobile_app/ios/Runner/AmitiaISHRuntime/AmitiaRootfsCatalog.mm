#import "AmitiaRootfsCatalog.h"

NSErrorDomain const AmitiaRootfsErrorDomain = @"AmitiaRootfsErrorDomain";

typedef NS_ENUM(NSInteger, AmitiaRootfsErrorCode) {
    AmitiaRootfsErrorNotConfigured = 1000,
    AmitiaRootfsErrorSourceInvalid,
    AmitiaRootfsErrorDownloadFailed,
    AmitiaRootfsErrorArchiveTooLarge,
    AmitiaRootfsErrorDigestMismatch,
    AmitiaRootfsErrorArchMismatch,
    AmitiaRootfsErrorArchiveInvalid,
    AmitiaRootfsErrorPathTraversal,
    AmitiaRootfsErrorSymlinkEscape,
    AmitiaRootfsErrorHardlinkEscape,
    AmitiaRootfsErrorExpandedSizeExceeded,
    AmitiaRootfsErrorValidationFailed,
    AmitiaRootfsErrorVersionDigestConflict,
    AmitiaRootfsErrorNotInstalled,
    AmitiaRootfsErrorCorrupted,
    AmitiaRootfsErrorRuntimeBusy,
    AmitiaRootfsErrorInstallCancelled,
    AmitiaRootfsErrorCommitFailed,
};

static const int64_t kMaxBytes = 150 * 1024 * 1024;
static NSString *const kSupportedSchema = @"1";
static NSString *const kAlpineID = @"alpine";

@interface AmitiaRootfsCatalogEntry ()
@property (nonatomic, copy, readwrite) NSString *schemaVersion;
@property (nonatomic, copy, readwrite) NSString *rootfsVersion;
@property (nonatomic, copy, readwrite) NSString *alpineVersion;
@property (nonatomic, copy, readwrite) NSString *guestArchitecture;
@property (nonatomic, copy, readwrite) NSString *artifactURL;
@property (nonatomic, copy, readwrite) NSString *sha256;
@property (nonatomic, readwrite) int64_t expectedSize;
@property (nonatomic, readwrite) AmitiaRootfsArchiveFormat archiveFormat;
@property (nonatomic, readwrite) AmitiaRootfsSourceType sourceType;
@property (nonatomic, copy, readwrite, nullable) NSString *bundleResource;
@end

@implementation AmitiaRootfsCatalogEntry

+ (BOOL)supportsSecureCoding { return YES; }

- (instancetype)initWithDictionary:(NSDictionary *)dict {
    self = [super init];
    if (!self) return nil;
    _schemaVersion = [dict[@"schemaVersion"] isKindOfClass:[NSString]] ? [dict[@"schemaVersion"] integerValue] == 1 ? kSupportedSchema : nil : nil;
    if (!_schemaVersion) {
        return nil;
    }
    _rootfsVersion = [dict[@"rootfsVersion"] isKindOfClass:[NSString]] ? dict[@"rootfsVersion"] : nil;
    _alpineVersion = [dict[@"alpineVersion"] isKindOfClass:[NSString]] ? dict[@"alpineVersion"] : nil;
    _guestArchitecture = [dict[@"architecture"] isKindOfClass:[NSString]] ? dict[@"architecture"] : nil;
    _artifactURL = [dict[@"artifactURL"] isKindOfClass:[NSString]] ? dict[@"artifactURL"] : nil;
    _sha256 = [dict[@"sha256"] isKindOfClass:[NSString]] ? dict[@"sha256"] : nil;
    _dict[@"expectedSize"] ? [dict[@"expectedSize"] longLongValue] : 0;
    NSString *fmt = [dict[@"archiveFormat"] isKindOfClass:[NSString]] ? dict[@"archiveFormat"] : nil;
    if ([fmt isEqualToString:@"tar.gz"]) _archiveFormat = AmitiaRootfsArchiveTarGz;
    else if ([fmt isEqualToString:@"tar.xz"]) _archiveFormat = AmitiaRootfsArchiveTarXz;
    else if ([fmt isEqualToString:@"tar.bz2"]) _archiveFormat = AmitiaRootfsArchiveTarBz2;
    else if ([fmt isEqualToString:@"tar"]) _archiveFormat = AmitiaRootfsArchiveTar;
    else return nil;
    NSString *src = [dict[@"sourceType"] isKindOfClass:[NSString]] ? dict[@"sourceType"] : nil;
    if ([src isEqualToString:@"bundled"]) _sourceType = AmitiaRootfsSourceBundled;
    else if ([src isEqualToString:@"remote"]) _sourceType = AmitiaRootfsSourceRemote;
    else return nil;
    _bundleResource = [dict[@"bundleResource"] isKindOfClass:[NSString]] ? dict[@"bundleResource"] : nil;
    return (self.isValid) ? self : nil;
}

- (BOOL)isValid {
    return (self.schemaVersion != nil
            && self.rootfsVersion.length > 0
            && self.alpineVersion.length > 0
            && self.guestArchitecture.length > 0
            && self.sha256.length == 64
            && self.expectedSize > 0
            && self.expectedSize <= kMaxBytes
            && self.artifactURL.length > 0
            && (self.sourceType != AmitiaRootfsSourceBundled || self.bundleResource.length > 0));
}

- (BOOL)matchesGuestArchitecture:(NSString *)runtimeArch error:(NSError **)error {
    if (![self.guestArchitecture isEqualToString:runtimeArch]) {
        if (error) {
            *error = [NSError errorWithDomain:AmitiaRootfsErrorDomain
                                         code:AmitiaRootfsErrorArchMismatch
                                     userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"rootfs arch %@ does not match runtime arch %@", self.guestArchitecture, runtimeArch]}];
        }
        return NO;
    }
    return YES;
}

- (void)encodeWithCoder:(NSCoder *)coder {
    [coder encodeObject:self.schemaVersion forKey:@"schemaVersion"];
    [coder encodeObject:self.rootfsVersion forKey:@"rootfsVersion"];
    [coder encodeObject:self.alpineVersion forKey:@"alpineVersion"];
    [coder encodeObject:self.guestArchitecture forKey:@"architecture"];
    [coder encodeInteger:self.expectedSize forKey:@"expectedSize"];
    [coder encodeObject:self.artifactURL forKey:@"artifactURL"];
    [coder encodeObject:self.bundleResource forKey:@"bundleResource"];
    [coder encodeObject:self.sha256 forKey:@"sha256"];
    [coder encodeInteger:(NSInteger)self.archiveFormat forKey:@"archiveFormat"];
    [coder encodeInteger:(NSInteger)self.sourceType forKey:@"sourceType"];
}

- (instancetype)initWithCoder:(NSCoder *)coder {
    NSDictionary *dict = @{
        @"schemaVersion": [coder decodeObjectForKey:@"schemaVersion"] ?: @"1",
        @"rootfsVersion": [coder decodeObjectForKey:@"rootfsVersion"] ?: @"",
        @"alpineVersion": [coder decodeObjectForKey:@"alpineVersion"] ?: @"",
        @"architecture": [coder decodeObjectForKey:@"architecture"] ?: @"",
        @"expectedSize": @([coder decodeInt64ForKey:@"expectedSize"]),
        @"artifactURL": [coder decodeObjectForKey:@"artifactURL"] ?: @"",
        @"bundleResource": [coder decodeObjectForKey:@"bundleResource"] ?: [NSNull null],
        @"sha256": [coder decodeObjectForKey:@"sha256"] ?: @"",
        @"archiveFormat": [coder decodeIntegerForKey:@"archiveFormat"] == 1 ? @"tar.xz"
                          : [coder decodeIntegerForKey:@"archiveFormat"] == 2 ? @"tar.bz2"
                          : [coder decodeIntegerForKey:@"archiveFormat"] == 3 ? @"tar"
                          : @"tar.gz",
        @"sourceType": [coder decodeIntegerForKey:@"sourceType"] == 1 ? @"remote" : @"bundled",
    };
    return [self initWithDictionary:dict];
}

@end

@interface AmitiaRootfsCatalog ()
@property (nonatomic, copy, readwrite) NSString *schemaVersion;
@property (nonatomic, copy, readwrite) NSArray<AmitiaRootfsCatalogEntry *> *entries;
@end

@implementation AmitiaRootfsCatalog

+ (instancetype)loadFromBundle:(NSError **)error {
    NSBundle *bundle = [NSBundle mainBundle];
    NSString *path = [bundle pathForResource:@"rootfs_catalog" ofType:@"json"];
    if (!path) {
        if (error) *error = [NSError errorWithDomain:AmitiaRootfsErrorDomain code:AmitiaRootfsErrorNotConfigured userInfo:@{NSLocalizedDescriptionKey: @"rootfs_catalog.json not found in bundle"}];
        return nil;
    }
    NSData *data = [NSData dataWithContentsOfFile:path options:0 error:error];
    if (!data) return nil;
    return [self loadFromData:data error:error];
}

+ (instancetype)loadFromData:(NSData *)data error:(NSError **)error NSDictionary *json = [NSJSONSerialization JSONObjectWithData:data options:0 error:error];
    if (![json isKindOfClass:[NSDictionary class]]) {
        if (error) *error = [NSError errorWithDomain:AmitiaRootfsErrorDomain code:AmitiaRootfsErrorArchiveInvalid userInfo:@{NSLocalizedDescriptionKey: @"invalid catalog JSON top-level"}];
        return nil;
    }
    NSMutableArray<AmitiaRootfsCatalogEntry *> *entries = [NSMutableArray array];
    NSArray *raw = json[@"entries"];
    if ([raw isKindOfClass:[NSArray class]]) {
        for (NSDictionary *item in raw) {
            if (![item isKindOfClass:[NSDictionary class]]) continue;
            AmitiaRootfsCatalogEntry *entry = [[AmitiaRootfsCatalogEntry alloc] initWithDictionary:item];
            if (entry) [entries addObject:entry];
        }
    }
    AmitiaRootfsCatalog *cat = [[self alloc] init];
    cat.schemaVersion = [json[@"schemaVersion"] isKindOfClass:[NSNumber]] ? [json[@"schemaVersion"] stringValue] : @"1";
    cat.entries = [entries copy];
    return cat;
}

- (AmitiaRootfsCatalogEntry *)entryForVersion:(NSString *)version arch:(NSString *)arch source:(AmitiaRootfsSourceType)source error:(NSError **)error {
    if (version.length == 0 || arch.length == 0) {
        if (error) *error = [NSError errorWithDomain:AmitiaRootfsErrorDomain code:AmitiaRootfsErrorSourceInvalid userInfo:@{NSLocalizedDescriptionKey: @"version and arch required"}];
        return nil;
    }
    for (AmitiaRootfsCatalogEntry *e in self.entries) {
        if (![e.rootfsVersion isEqualToString:version]) continue;
        if (e.sourceType != source) continue;
        if (![e matchesGuestArchitecture:arch error:nil]) continue;
        return e;
    }
    if (error) *error = [NSError errorWithDomain:AmitiaRootfsErrorDomain code:AmitiaRootfsErrorNotConfigured userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"no catalog entry for %@/%@ source=%ld", version, arch, (long)source]}];
    return nil;
}

@end
