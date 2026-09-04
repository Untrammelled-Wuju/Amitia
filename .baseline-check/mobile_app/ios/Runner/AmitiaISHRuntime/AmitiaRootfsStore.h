#import <Foundation/Foundation.h>
#import "AmitiaRootfsCatalog.h"

NS_ASSUME_NONNULL_BEGIN

extern NSErrorDomain const AmitiaRootfsErrorDomain;

@interface AmitiaRootfsVersionMetadata : NSObject <NSSecureCoding>

@property (nonatomic, copy, readonly) NSString *schemaVersion;
@property (nonatomic, copy, readonly) NSString *installationId;
@property (nonatomic, copy, readonly) NSString *rootfsVersion;
@property (nonatomic, copy, readonly) NSString *alpineVersion;
@property (nonatomic, copy, readonly) NSString *guestArchitecture;
@property (nonatomic, copy, readonly) NSString *archiveDigest;
@property (nonatomic, copy, readonly) NSString *installedAt;
@property (nonatomic, readonly) BOOL validated;
@property (nonatomic, readonly) int64_t archiveSizeBytes;
@property (nonatomic, copy, readonly) NSString *archiveFormat;
@property (nonatomic, copy, readonly) NSString *sourceType;
@property (nonatomic, copy, readonly) NSString *sourceRef;

@end

@interface AmitiaRootfsStore : NSObject

@property (nonatomic, copy, readonly) NSString *rootDirectory;

- (instancetype)initWithRootDirectory:(NSString *)path NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

- (void)shutdown;

- (NSArray<NSString *> *)listInstalledVersions;
- (AmitiaRootfsVersionMetadata *_Nullable)metadataForVersion:(NSString *)version;
- (AmitiaRootfsVersionMetadata *_Nullable)activeMetadata;

- (NSString *_Nullable)pathForInstallation:(NSString *)installationId NSError:(NSError **)error;

- (BOOL)commitStaging:(NSString *)stagingPath
           spec:(AmitiaRootfsCatalogEntry *)entry
         installationId:(NSString *)installationId
             validated:(BOOL)validated
                 error:(NSError **)error;

- (BOOL)activateVersion:(NSString *)installationId error:(NSError **)error;
- (BOOL)removeVersion:(NSString *)installationId force:(BOOL)force error:(NSError **)error;

- (BOOL)verifyVersion:(NSString *)installationId error:(NSError **)error;

@end

NS_ASSUME_NONNULL_END
