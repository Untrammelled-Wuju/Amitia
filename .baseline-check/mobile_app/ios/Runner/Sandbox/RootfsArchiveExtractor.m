#import "RootfsArchiveExtractor.h"
#import <compression.h>
#import <sys/stat.h>

NSErrorDomain const RootfsArchiveExtractorErrorDomain = @"com.amitia.RootfsArchiveExtractor";

static const NSUInteger kDefaultMaxEntries = 250000;
static const int64_t kDefaultMaxExtractedBytes = 2LL * 1024 * 1024 * 1024;
static const size_t kStreamBufferSize = 1024 * 1024;

@interface RootfsArchiveExtractor ()
@property (nonatomic, readwrite) NSUInteger maxEntries;
@property (nonatomic, readwrite) int64_t maxExtractedBytes;
@end

@implementation RootfsArchiveExtractor

- (instancetype)initWithMaxEntries:(NSUInteger)maxEntries maxExtractedBytes:(int64_t)maxBytes {
    self = [super init];
    if (self) {
        _maxEntries = maxEntries;
        _maxExtractedBytes = maxBytes;
        _cancellationRequested = NO;
    }
    return self;
}

+ (instancetype)defaultExtractor {
    return [[RootfsArchiveExtractor alloc] initWithMaxEntries:kDefaultMaxEntries maxExtractedBytes:kDefaultMaxExtractedBytes];
}

- (BOOL)extractArchiveAtURL:(NSURL *)archiveURL toDirectory:(NSURL *)destinationURL error:(NSError **)error {
    if (!archiveURL || !archiveURL.isFileURL) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorInvalidArchive userInfo:@{NSLocalizedDescriptionKey: @"Invalid archive URL"}];
        return NO;
    }

    NSFileManager *fm = [NSFileManager defaultManager];
    NSError *dirErr = nil;
    if (![fm createDirectoryAtURL:destinationURL withIntermediateDirectories:YES attributes:nil error:&dirErr]) {
        if (error) *error = dirErr;
        return NO;
    }

    NSString *destPath = destinationURL.URLByResolvingSymlinksInPath.path;
    NSURL *destURLResolved = [NSURL fileURLWithPath:destPath];

    if (![self validateArchivePath:archiveURL error:error]) return NO;

    NSInputStream *input = [NSInputStream inputStreamWithURL:archiveURL];
    [input open];
    if (input.streamError) {
        if (error) *error = input.streamError;
        return NO;
    }
    if (input.streamStatus == NSStreamStatusError) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorIOFailure userInfo:nil];
        [input close];
        return NO;
    }

    BOOL ok = [self extractZipStream:input toDestination:destURLResolved basePath:destPath error:error];
    [input close];
    return ok;
}

- (BOOL)extractZipStream:(NSInputStream *)input toDestination:(NSURL *)destURL basePath:(NSString *)basePath error:(NSError **)error {
    NSFileManager *fm = [NSFileManager defaultManager];
    NSMutableData *entryNameBuffer = [NSMutableData data];
    NSUInteger entryCount = 0;
    int64_t totalExtracted = 0;
    uint32_t signature = 0;
    NSInteger n;

    uint8_t buf[4];
    n = [input read:buf maxLength:4];
    if (n != 4) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorArchiveCorrupt userInfo:nil];
        return NO;
    }
    signature = (uint32_t)buf[0] | ((uint32_t)buf[1] << 8) | ((uint32_t)buf[2] << 16) | ((uint32_t)buf[3] << 24);
    if (signature != 0x04034b50 && signature != 0x02014b50) {
        if (signature != 0x06054b50) {
            if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorInvalidArchive userInfo:nil];
            return NO;
        }
        return YES;
    }

    [input close];

    if (self.cancellationRequested) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorCancelled userInfo:nil];
        return NO;
    }

    if (self.cancellationRequested) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorCancelled userInfo:nil];
        return NO;
    }

    if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorUnsupportedEntryType userInfo:@{NSLocalizedDescriptionEntryType: @"Full ZIP extraction requires libarchive integration"}];
    return NO;
}

- (BOOL)validateArchivePath:(NSURL *)archiveURL error:(NSError **)error {
    if (!archiveURL.isFileURL) return NO;
    NSString *path = archiveURL.path;
    if ([path rangeOfString:@".."].location != NSNotFound) {
        if (error) *error = [NSError errorWithDomain:RootfsArchiveExtractorErrorDomain code:RootfsExtractionErrorPathTraversal userInfo:nil];
        return NO;
    }
    return YES;
}

@end
