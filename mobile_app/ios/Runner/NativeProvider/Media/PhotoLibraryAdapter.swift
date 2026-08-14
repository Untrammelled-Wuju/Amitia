import Foundation
import Photos
import UIKit

public class PhotoLibraryAdapter: NSObject {
    public static let shared = PhotoLibraryAdapter()

    private let imageManager = PHImageManager.default()

    private override init() {
        super.init()
    }

    public func authorizationStatus() -> PHAuthorizationStatus {
        return PHPhotoLibrary.authorizationStatus(for: .readWrite)
    }

    public func requestPermission() async -> PHAuthorizationStatus {
        let current = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        if current != .notDetermined {
            return current
        }
        return await PHPhotoLibrary.requestAuthorization(for: .readWrite)
    }

    public func fetchAssets(mediaType: PHAssetMediaType = .image, limit: Int = 100, offset: Int = 0) -> [PHAsset] {
        let fetchOptions = PHFetchOptions()
        fetchOptions.sortDescriptors = [NSSortDescriptor(key: "creationDate", ascending: false)]
        fetchOptions.fetchLimit = limit
        fetchOptions.fetchOffset = offset

        let result = PHAsset.fetchAssets(with: mediaType, options: fetchOptions)
        var assets: [PHAsset] = []
        result.enumerateObjects { asset, _, _ in
            assets.append(asset)
        }
        return assets
    }

    public func fetchAlbums() -> [PHAssetCollection] {
        var albums: [PHAssetCollection] = []

        let smartAlbums = PHAssetCollection.fetchAssetCollections(with: .smartAlbum, subtype: .any, options: nil)
        smartAlbums.enumerateObjects { collection, _, _ in
            albums.append(collection)
        }

        let userAlbums = PHAssetCollection.fetchAssetCollections(with: .album, subtype: .any, options: nil)
        userAlbums.enumerateObjects { collection, _, _ in
            albums.append(collection)
        }

        return albums
    }

    public func requestImage(for asset: PHAsset, targetSize: CGSize = CGSize(width: 300, height: 300), completion: @escaping (UIImage?) -> Void) {
        let options = PHImageRequestOptions()
        options.deliveryMode = .opportunistic
        options.isSynchronous = false

        imageManager.requestImage(for: asset, targetSize: targetSize, contentMode: .aspectFill, options: options) { image, _ in
            completion(image)
        }
    }

    public func saveImage(_ image: Data, completion: @escaping (Bool, Error?) -> Void) {
        PHPhotoLibrary.shared().performChanges({
            let creationRequest = PHAssetCreationRequest.forAsset()
            creationRequest.addResource(with: .photo, data: image, options: nil)
        }, completionHandler: completion)
    }

    public func deleteAssets(_ assets: [PHAsset], completion: @escaping (Bool, Error?) -> Void) {
        PHPhotoLibrary.shared().performChanges({
            PHAssetChangeRequest.deleteAssets(assets as NSArray)
        }, completionHandler: completion)
    }
}
