import '../models/schema_ui_types.dart';

class DataSourceResult {
  final dynamic data;
  final bool isLoading;
  final String? error;

  const DataSourceResult({
    this.data,
    this.isLoading = false,
    this.error,
  });

  DataSourceResult copyWith({
    dynamic data,
    bool? isLoading,
    String? error,
  }) {
    return DataSourceResult(
      data: data ?? this.data,
      isLoading: isLoading ?? this.isLoading,
      error: error ?? this.error,
    );
  }
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
}

class DataSourceLoader {
  final DataSourceFetcher fetcher;

  const DataSourceLoader({required this.fetcher});

  Future<DataSourceResult> load(DataSourceRequest request) async {
    try {
      final data = await fetcher(request);
      return DataSourceResult(data: data);
    } catch (e) {
      return DataSourceResult(error: e.toString());
    }
  }

  bool requiresFetch(SchemaUIDataSource? ds) {
    if (ds == null) return false;
    return ds.type == 'query' || ds.type == 'runtime';
  }
}
