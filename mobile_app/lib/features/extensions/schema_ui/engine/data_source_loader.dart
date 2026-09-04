import '../models/schema_ui_types.dart';

class DataSourceResult {
  final dynamic data;
  final bool isLoading;
  final String? error;
  final DateTime? fetchedAt;

  const DataSourceResult({
    this.data,
    this.isLoading = false,
    this.error,
    this.fetchedAt,
  });

  DataSourceResult copyWith({
    dynamic data,
    bool? isLoading,
    String? error,
    DateTime? fetchedAt,
  }) {
    return DataSourceResult(
      data: data ?? this.data,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
      fetchedAt: fetchedAt ?? this.fetchedAt,
    );
  }

  bool get hasData => data != null && error == null;
  bool get hasError => error != null;
}

typedef DataSourceFetcher = Future<dynamic> Function(DataSourceRequest request);

class DataSourceRequest {
  final SchemaUIDataSource dataSource;
  final Map<String, dynamic> input;
  final String extensionId;
  final String contributionId;

  const DataSourceRequest({
    required this.dataSource,
    this.input = const {},
    required this.extensionId,
    required this.contributionId,
  });

  String get cacheKey => '${extensionId}:${contributionId}:${dataSource.type}:${dataSource.id}:${input.toString()}';
}

class DataSourceEntry {
  DataSourceResult result;
  final DateTime cachedAt;
  final Duration ttl;

  DataSourceEntry({required this.result, required this.cachedAt, this.ttl = const Duration(minutes: 5)});

  bool get isExpired => DateTime.now().difference(cachedAt) > ttl;
}

class DataSourceLoader {
  final DataSourceFetcher fetcher;
  final Map<String, DataSourceEntry> _cache = {};
  final Map<String, CancelToken> _inFlight = {};

  DataSourceLoader({required this.fetcher});

  Future<DataSourceResult> load(DataSourceRequest request) async {
    final key = request.cacheKey;
    final cached = _cache[key];
    if (cached != null && !cached.isExpired) {
      return cached.result;
    }
    final cancelToken = CancelToken();
    _inFlight[key] = cancelToken;
    try {
      final data = await fetcher(request);
      if (cancelToken.isCancelled) {
        return const DataSourceResult(error: 'Request cancelled');
      }
      final result = DataSourceResult(data: data, fetchedAt: DateTime.now());
      _cache[key] = DataSourceEntry(result: result, cachedAt: DateTime.now());
      return result;
    } catch (e) {
      if (cancelToken.isCancelled) {
        return const DataSourceResult(error: 'Request cancelled');
      }
      final result = DataSourceResult(error: e.toString());
      _cache[key] = DataSourceEntry(result: result, cachedAt: DateTime.now(), ttl: const Duration(seconds: 30));
      return result;
    } finally {
      _inFlight.remove(key);
    }
  }

  void cancel(String extensionId, String contributionId) {
    final prefix = '$extensionId:$contributionId:';
    final keysToRemove = _inFlight.keys.where((k) => k.startsWith(prefix)).toList();
    for (final key in keysToRemove) {
      _inFlight[key]?.cancel();
      _inFlight.remove(key);
    }
  }

  void invalidate(String extensionId, String contributionId) {
    final prefix = '$extensionId:$contributionId:';
    _cache.removeWhere((key, _) => key.startsWith(prefix));
  }

  void clearCache() {
    _cache.clear();
  }

  bool requiresFetch(SchemaUIDataSource? ds) {
    if (ds == null) return false;
    return ds.id.trim().isNotEmpty;
  }
}

class CancelToken {
  bool _cancelled = false;
  bool get isCancelled => _cancelled;
  void cancel() => _cancelled = true;
}
