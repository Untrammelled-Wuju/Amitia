#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, AmitiaRootfsSourceType) {
    AmitiaRootfsSourceBundled = 0,
    AmitiaRootfsSourceRemote = 1,
};

typedef NS_ENUM(NSInteger, AmitiaRootfsArchiveFormat) {
    AmitiaRootfsArchiveTarGz = 0,
    AmitiaRootfsArchiveTarXz = 1,
    AmitiaRootfsArchiveTarBz2 = 2,
    AmitiaRootfsArchiveTar = 3,
};

@interface AmitiaRootfsCatalogEntry : NSObject <NSSecureCoding>

@property (nonatomic, copy, readonly) NSString *schemaVersion;
@property (nonatomic, copy, readonly) NSString *rootfsVersion;
@property (nonatomic, copy, readonly) NSString *alpineVersion;
@property (nonatomic, copy, readonly) NSString *guestArchitecture;
@property (nonatomic, copy, readonly) NSString *artifactURL;
@property (nonatomic, copy, readonly) NSString *sha256;
@property (nonatomic, readonly) int64_t expectedSize;
@property (nonatomic, readonly) AmitiaRootfsArchiveFormat archiveFormat;
@property (nonatomic, readonly) AmitiaRootfsSourceType sourceType;
@property (nonatomic, copy, readonly, nullable) NSString *bundleResource;

- (instancetype)initWithDictionary:(NSDictionary *)dict;
- (BOOL)matchesGuestArchitecture:(NSString *)runtimeArch error:(NSError **)error;

@end

@interface AmitiaRootfsCatalog : NSObject

@property (nonatomic, copy, readonly) NSString *schemaVersion;
@property (nonatomic, copy, readonly) NSArray<AmitiaRootfsCatalogEntry *> *entries;

+ (instancetype)loadFromBundle:(NSError **)error;
+ (instancetype)loadFromData:(NSData *)data error:(NSError **)error;
- (AmitiaRootfsCatalogEntry *_Nullable)entryForVersion:(NSString *)version
                                                 arch:(NSString *)arch
                                               source:(AmitiaRootfsSourceType)source
                                                error:(NSError **)error;

@end

NS_ASSUME_NONNULL_END
