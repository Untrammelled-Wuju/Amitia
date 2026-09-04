#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, RootfsExtractionError) {
    RootfsExtractionErrorNone = 0,
    RootfsExtractionErrorInvalidArchive = 3000,
    RootfsExtractionErrorArchiveCorrupt,
    RootfsExtractionErrorPathTraversal,
    RootfsExtractionErrorSymlinkEscape,
    RootfsExtractionErrorHardlinkEscape,
    RootfsExtractionErrorUnsupportedEntryType,
    RootfsExtractionErrorTooManyEntries,
    RootfsExtractionErrorSizeExceeded,
    RootfsExtractionErrorIOFailure,
    RootfsExtractionErrorCancelled
};

@interface RootfsArchiveExtractor : NSObject

@property (nonatomic, readonly) NSUInteger maxEntries;
@property (nonatomic, readonly) int64_t maxExtractedBytes;
@property (atomic) BOOL cancellationRequested;

- (instancetype)initWithMaxEntries:(NSUInteger)maxEntries maxExtractedBytes:(int64_t)maxBytes;

- (BOOL)extractArchiveAtURL:(NSURL *)archiveURL
               toDirectory:(NSURL *)destinationURL
                     error:(NSError *_Nullable *_Nullable)error;

+ (instancetype)defaultExtractor;

@end

extern NSErrorDomain const RootfsArchiveExtractorErrorDomain;

NS_ASSUME_NONNULL_END
