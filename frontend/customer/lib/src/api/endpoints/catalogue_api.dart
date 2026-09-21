import 'package:goklay_core/goklay_core.dart';

import '../api_page.dart';
import '../models/catalogue.dart';

/// A shop's menu, as a customer sees it.
class CatalogueApi {
  /// Creates the API against [client].
  const CatalogueApi(this._client);

  final GoklayApiClient _client;

  /// The whole catalogue for one shop.
  Future<ApiPage<Menu>> menu(String merchantId) async {
    final ApiResponse response = await _client.get(
      '/v1/catalogue/$merchantId/menu',
    );
    return ApiPage<Menu>.of(response, Menu.fromJson(response.asObject));
  }

  /// One item, with its variant and add-on groups.
  Future<ApiPage<PublicItem>> item({
    required String merchantId,
    required String itemId,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/catalogue/$merchantId/items/$itemId',
    );
    return ApiPage<PublicItem>.of(
      response,
      PublicItem.fromJson(response.asObject),
    );
  }

  /// One bundle.
  Future<ApiPage<Combo>> combo({
    required String merchantId,
    required String comboId,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/catalogue/$merchantId/combos/$comboId',
    );
    return ApiPage<Combo>.of(response, Combo.fromJson(response.asObject));
  }
}
