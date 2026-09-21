import 'package:goklay_core/goklay_core.dart';

import '../models/discovery.dart';

/// Finding shops, and placing the customer on the map.
///
/// `level` is the one parameter worth explaining: it is the rung of the radius
/// ladder to search at, and the app never invents a value for it. It sends 0
/// on a first search and then whatever [DiscoveryExpansion.nextLevel] said,
/// and only when [DiscoveryExpansion.canExpand] was true. The ladder, its
/// radii, its surcharge (D2) and the division ceiling (D3) are all the
/// server's (2.9).
class DiscoveryApi {
  /// Creates the API against [client].
  const DiscoveryApi(this._client);

  final GoklayApiClient _client;

  /// Searches around a point.
  Future<ApiPage<DiscoverySearch>> search({
    required double lat,
    required double lng,
    int level = 0,
    String? type,
    String? query,
    int limit = 20,
    int offset = 0,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/discovery/merchants',
      query: <String, String>{
        'lat': '$lat',
        'lng': '$lng',
        'level': '$level',
        if (type != null && type.isNotEmpty) 'type': type,
        if (query != null && query.isNotEmpty) 'q': query,
        'limit': '$limit',
        'offset': '$offset',
      },
    );
    return ApiPage<DiscoverySearch>.of(
      response,
      DiscoverySearch.fromJson(response.asObject),
    );
  }

  /// Places a coordinate in the administrative hierarchy.
  Future<ApiPage<Area>> resolve({
    required double lat,
    required double lng,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/geo/resolve',
      query: <String, String>{'lat': '$lat', 'lng': '$lng'},
    );
    return ApiPage<Area>.of(response, Area.fromJson(response.asObject));
  }
}
