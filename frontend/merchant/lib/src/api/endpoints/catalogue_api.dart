import 'package:goklay_core/goklay_core.dart';

import '../models/catalogue.dart';

/// The owner's view of their own catalogue.
///
/// Every path is scoped to a merchant id, and the server checks that the
/// caller owns that shop — so one owner cannot touch another's menu however
/// the app is driven.
class MerchantCatalogueApi {
  /// Creates the API against [client].
  const MerchantCatalogueApi(this._client);

  final GoklayApiClient _client;

  /// What a catalogue of this shop's type may contain.
  Future<ApiPage<CatalogueCapabilities>> capabilities(
    String merchantId,
  ) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/catalogue/capabilities',
    );
    return ApiPage<CatalogueCapabilities>.of(
      response,
      CatalogueCapabilities.fromJson(response.asObject),
    );
  }

  /// The shop's sections.
  Future<ApiPage<List<OwnerCategory>>> categories(String merchantId) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/catalogue/categories',
    );
    return ApiPage<List<OwnerCategory>>.of(
      response,
      _list(response.asObject['categories'], OwnerCategory.fromJson),
    );
  }

  /// Adds a section.
  Future<OwnerCategory> addCategory({
    required String merchantId,
    required String name,
    int sortOrder = 0,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants/$merchantId/catalogue/categories',
      body: <String, Object?>{'name': name, 'sort_order': sortOrder},
    );
    return OwnerCategory.fromJson(response.asObject);
  }

  /// Shows or hides a section. Hiding takes its items off the menu with it,
  /// which the server does — the app does not walk the items itself.
  Future<OwnerCategory> setCategoryActive({
    required String merchantId,
    required String categoryId,
    required bool active,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/$merchantId/catalogue/categories/$categoryId/active',
      body: <String, Object?>{'active': active},
    );
    return OwnerCategory.fromJson(response.asObject);
  }

  /// The shop's items.
  Future<ApiPage<List<OwnerItem>>> items(String merchantId) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/catalogue/items',
    );
    return ApiPage<List<OwnerItem>>.of(
      response,
      _list(response.asObject['items'], OwnerItem.fromJson),
    );
  }

  /// Adds an item.
  ///
  /// The price goes as a minor-unit integer, which is the only form money
  /// crosses the wire in. The app never parses a decimal into one — the field
  /// takes poisha and the server formats every figure that comes back.
  Future<OwnerItem> addItem({
    required String merchantId,
    required String categoryId,
    required String name,
    required int priceMinor,
    String description = '',
    String unit = '',
    String packSize = '',
    String brand = '',
    bool requiresPrescription = false,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants/$merchantId/catalogue/items',
      body: <String, Object?>{
        'category_id': categoryId,
        'name': name,
        'price_minor': priceMinor,
        'description': description,
        if (unit.isNotEmpty) 'unit': unit,
        if (packSize.isNotEmpty) 'pack_size': packSize,
        if (brand.isNotEmpty) 'brand': brand,
        if (requiresPrescription) 'requires_prescription': true,
      },
    );
    return OwnerItem.fromJson(response.asObject);
  }

  /// Turns an item on or off.
  Future<OwnerItem> setItemActive({
    required String merchantId,
    required String itemId,
    required bool active,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/$merchantId/catalogue/items/$itemId/active',
      body: <String, Object?>{'active': active},
    );
    return OwnerItem.fromJson(response.asObject);
  }

  /// Sets the shelf count. Only offered when the capabilities say this shop
  /// type counts one.
  Future<OwnerItem> setStock({
    required String merchantId,
    required String itemId,
    required int quantity,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/$merchantId/catalogue/items/$itemId/stock',
      body: <String, Object?>{'quantity': quantity},
    );
    return OwnerItem.fromJson(response.asObject);
  }

  /// The shop's bundles.
  Future<ApiPage<List<OwnerCombo>>> combos(String merchantId) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/catalogue/combos',
    );
    return ApiPage<List<OwnerCombo>>.of(
      response,
      _list(response.asObject['combos'], OwnerCombo.fromJson),
    );
  }

  /// Turns a bundle on or off.
  Future<OwnerCombo> setComboActive({
    required String merchantId,
    required String comboId,
    required bool active,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/merchants/$merchantId/catalogue/combos/$comboId/active',
      body: <String, Object?>{'active': active},
    );
    return OwnerCombo.fromJson(response.asObject);
  }

  static List<T> _list<T>(
    Object? raw,
    T Function(Map<String, Object?>) parse,
  ) => <T>[
    if (raw is List<Object?>)
      for (final Object? entry in raw)
        if (entry is Map<String, Object?>) parse(entry),
  ];
}
