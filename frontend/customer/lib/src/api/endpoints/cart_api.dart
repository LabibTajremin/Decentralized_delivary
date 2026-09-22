import 'package:goklay_core/goklay_core.dart';

import '../models/cart.dart';

/// One chosen option, as the client is allowed to express it.
///
/// Ids only. The price of an add-on is never sent: a client that could state
/// a price is a client that could state the wrong one, so the server looks
/// every choice up and prices the line itself (2.9).
class CartChoice {
  /// Creates a choice.
  const CartChoice({required this.groupId, required this.optionId});

  /// The group it came from.
  final String groupId;

  /// The option picked.
  final String optionId;

  /// The wire form.
  Map<String, Object?> toJson() => <String, Object?>{
    'group_id': groupId,
    'option_id': optionId,
  };
}

/// The cart. Every call here returns the whole revalidated cart, because the
/// server rechecks prices, stock and the shop's hours on each one.
class CartApi {
  /// Creates the API against [client].
  const CartApi(this._client);

  final GoklayApiClient _client;

  /// Reads the cart.
  Future<ApiPage<Cart>> cart() async {
    final ApiResponse response = await _client.get('/v1/cart');
    return ApiPage<Cart>.of(response, Cart.fromJson(response.asObject));
  }

  /// Adds an item or a combo.
  Future<Cart> add({
    required String merchantId,
    required String kind,
    required String targetId,
    required int quantity,
    List<CartChoice> choices = const <CartChoice>[],
    String note = '',
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/cart/items',
      body: <String, Object?>{
        'merchant_id': merchantId,
        'kind': kind,
        'target_id': targetId,
        'quantity': quantity,
        'choices': <Map<String, Object?>>[
          for (final CartChoice choice in choices) choice.toJson(),
        ],
        'note': note,
      },
    );
    return Cart.fromJson(response.asObject);
  }

  /// Changes a line's quantity. Zero removes it, which is the server's rule,
  /// not a special case the app implements.
  Future<Cart> setQuantity({
    required String lineId,
    required int quantity,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/cart/lines/$lineId',
      body: <String, Object?>{'quantity': quantity},
    );
    return Cart.fromJson(response.asObject);
  }

  /// Removes a line.
  Future<Cart> removeLine(String lineId) async {
    final ApiResponse response = await _client.send(
      'DELETE',
      '/v1/cart/lines/$lineId',
    );
    return Cart.fromJson(response.asObject);
  }

  /// Binds the cart to a delivery address, which is what makes the fee and
  /// the total real: until there is a destination there is no distance, and
  /// without a distance there is no delivery fee to quote.
  Future<Cart> setAddress({
    required String addressId,
    required double lat,
    required double lng,
  }) async {
    final ApiResponse response = await _client.send(
      'PUT',
      '/v1/cart/address',
      body: <String, Object?>{
        'address_id': addressId,
        'lat': lat,
        'lng': lng,
      },
    );
    return Cart.fromJson(response.asObject);
  }

  /// Empties the cart.
  Future<void> clear() => _client.send('DELETE', '/v1/cart');
}
