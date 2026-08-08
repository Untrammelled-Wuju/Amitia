#import "RootfsDescriptor.h"

@implementation RootfsDescriptor

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                  digestSHA256:(NSString *)digest
                      rootfsURL:(NSURL *)rootfsURL
                     sourceType:(RootfsSourceType)sourceType {
    return [self initWithVersion:version
                    architecture:architecture
                   digestSHA256:digest
                       rootfsURL:rootfsURL
                        mountURL:nil
                      sourceType:sourceType
                          format:RootfsFormatUnknown
              packageDigestSHA256:nil
                   formatVersion:nil
                    manifestPath:nil
                           state:RootfsStateNotInstalled];
}

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                  digestSHA256:(NSString *)digest
                      rootfsURL:(NSURL *)rootfsURL
                       mountURL:(NSURL *)mountURL
                     sourceType:(RootfsSourceType)sourceType
                         format:(RootfsFormat)format
             packageDigestSHA256:(NSString *)packageDigest
                  formatVersion:(NSString *)formatVersion
                   manifestPath:(NSString *)manifestPath
                          state:(RootfsState)state {
    self = [super init];
    if (self) {
        _version = [version copy];
        _architecture = [architecture copy];
        _digestSHA256 = [digest copy];
        _rootfsURL = rootfsURL;
        _mountURL = mountURL;
        _sourceType = sourceType;
        _format = format;
        _packageDigestSHA256 = [packageDigest copy];
        _formatVersion = [formatVersion copy];
        _manifestPath = [manifestPath copy];
        _state = state;
    }
    return self;
}

- (BOOL)isValidDescriptor {
    if (self.version.length == 0) return NO;
    if (self.architecture.length == 0) return NO;
    if (self.digestSHA256.length != 64) return NO;
    if (self.rootfsURL == nil) return NO;
    NSCharacterSet *hexSet = [NSCharacterSet characterSetWithCharactersInString:@"0123456789abcdef"];
    if ([self.digestSHA256 rangeOfCharacterFromSet:[hexSet invertedSet]].location != NSNotFound) return NO;
    return YES;
}

- (BOOL)verifyLayoutPresent {
    if (!self.rootfsURL) return NO;

    if (self.format == RootfsFormatISHFakeFS || self.mountURL != nil) {
        return [self verifyISHFakeFSLayout];
    }

    NSFileManager *fm = [NSFileManager defaultManager];
    NSArray *required = @[@"bin", @"etc", @"usr", @"var", @"tmp", @"home"];
    for (NSString *dir in required) {
        NSString *path = [self.rootfsURL.path stringByAppendingPathComponent:dir];
        BOOL isDir = NO;
        if (![fm fileExistsAtPath:path isDirectory:&isDir] || !isDir) {
            return NO;
        }
    }
    return YES;
}

- (BOOL)verifyISHFakeFSLayout {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSURL *base = self.mountURL ?: self.rootfsURL;
    BOOL isDir = NO;

    NSString *dataPath = [base.path stringByAppendingPathComponent:@"data"];
    if (![fm fileExistsAtPath:dataPath isDirectory:&isDir] || !isDir) return NO;

    NSString *metaDbPath = [self.rootfsURL.path stringByAppendingPathComponent:@"meta.db"];
    if (![fm fileExistsAtPath:metaDbPath isDirectory:&isDir] || isDir) return NO;

    NSString *binSh = [dataPath stringByAppendingPathComponent:@"bin/sh"];
    if (![fm fileExistsAtPath:binSh]) return NO;

    NSString *etcPath = [dataPath stringByAppendingPathComponent:@"etc"];
    if (![fm fileExistsAtPath:etcPath isDirectory:&isDir] || !isDir) return NO;

    NSString *usrPath = [dataPath stringByAppendingPathComponent:@"usr"];
    if (![fm fileExistsAtPath:usrPath isDirectory:&isDir] || !isDir) return NO;

    NSError *err = nil;
    NSNumber *metaSize = nil;
    if ([self.rootfsURL getResourceValue:&metaSize forKey:NSFileSizeKey error:&err]) {
        if (metaSize.longLongValue <= 0) return NO;
    } else {
        if (err) return NO;
    }

    return YES;
}

- (instancetype)descriptorBySettingState:(RootfsState)state {
    return [[RootfsDescriptor alloc] initWithVersion:self.version
                                        architecture:self.architecture
                                       digestSHA256:self.digestSHA256
                                           rootfsURL:self.rootfsURL
                                            mountURL:self.mountURL
                                          sourceType:self.sourceType
                                              format:self.format
                                  packageDigestSHA256:self.packageDigestSHA256
                                       formatVersion:self.formatVersion
                                        manifestPath:self.manifestPath
                                               state:state];
}

+ (NSString *)stringFromState:(RootfsState)state {
    switch (state) {
        case RootfsStateUnknown: return @"unknown";
        case RootfsStateNotInstalled: return @"not_installed";
        case RootfsStateInstalling: return @"installing";
        case RootfsStateInstalled: return @"installed";
        case RootfsStateCorrupt: return @"corrupt";
        case RootfsStateFailed: return @"failed";
    }
    return @"unknown";
}

+ (NSString *)stringFromSourceType:(RootfsSourceType)type {
    switch (type) {
        case RootfsSourceTypeUnknown: return @"unknown";
        case RootfsSourceTypeBundled: return @"bundled";
        case RootfsSourceTypeRemoteOfficial: return @"remote_official";
        case RootfsSourceTypeUserImported: return @"user_imported";
        case RootfsSourceTypeLegacy: return @"legacy";
    }
    return @"unknown";
}

+ (NSString *)stringFromFormat:(RootfsFormat)format {
    switch (format) {
        case RootfsFormatUnknown: return @"unknown";
        case RootfsFormatISHFakeFS: return @"ish_fakefs";
    }
    return @"unknown";
}

@end
