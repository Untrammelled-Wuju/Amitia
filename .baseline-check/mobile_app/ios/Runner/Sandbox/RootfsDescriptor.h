#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsState) {
    RootfsStateUnknown = 0,
    RootfsStateNotInstalled,
    RootfsStateInstalling,
    RootfsStateInstalled,
    RootfsStateCorrupt,
    RootfsStateFailed
};

typedef NS_ENUM(NSInteger, RootfsSourceType) {
    RootfsSourceTypeUnknown = 0,
    RootfsSourceTypeBundled,
    RootfsSourceTypeRemoteOfficial,
    RootfsSourceTypeUserImported,
    RootfsSourceTypeLegacy
};

typedef NS_ENUM(NSInteger, RootfsFormat) {
    RootfsFormatUnknown = 0,
    RootfsFormatISHFakeFS
};

@interface RootfsDescriptor : NSObject

@property (nonatomic, copy, readonly) NSString *version;
@property (nonatomic, copy, readonly) NSString *architecture;
@property (nonatomic, copy, readonly) NSString *digestSHA256;
@property (nonatomic, copy, readonly) NSURL *rootfsURL;
@property (nonatomic, copy, readonly, nullable) NSURL *mountURL;
@property (nonatomic, readonly) RootfsSourceType sourceType;
@property (nonatomic, readonly) RootfsState state;
@property (nonatomic, readonly) RootfsFormat format;
@property (nonatomic, copy, readonly, nullable) NSString *packageDigestSHA256;
@property (nonatomic, copy, readonly, nullable) NSString *formatVersion;
@property (nonatomic, copy, readonly, nullable) NSString *manifestPath;

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                  digestSHA256:(NSString *)digest
                      rootfsURL:(NSURL *)rootfsURL
                     sourceType:(RootfsSourceType)sourceType;

- (instancetype)initWithVersion:(NSString *)version
                   architecture:(NSString *)architecture
                  digestSHA256:(NSString *)digest
                      rootfsURL:(NSURL *)rootfsURL
                      mountURL:(NSURL *)mountURL
                     sourceType:(RootfsSourceType)sourceType
                         format:(RootfsFormat)format
              packageDigestSHA256:(nullable NSString *)packageDigest
                  formatVersion:(nullable NSString *)formatVersion
                   manifestPath:(nullable NSString *)manifestPath
                          state:(RootfsState)state;

- (BOOL)isValidDescriptor;
- (BOOL)verifyLayoutPresent;
- (BOOL)verifyISHFakeFSLayout;
- (instancetype)descriptorBySettingState:(RootfsState)state;

+ (NSString *)stringFromState:(RootfsState)state;
+ (NSString *)stringFromSourceType:(RootfsSourceType)type;
+ (NSString *)stringFromFormat:(RootfsFormat)format;

@end

NS_ASSUME_NONNULL_END
