import 'package:goklay_core/goklay_core.dart';

import '../models/order.dart';

/// The shop's order board.
///
/// Four of these are transitions, and the app offers each one only when the
/// order's `next_actions` names it. That list comes from the order state
/// machine itself (P11), so a shop is never shown a button the server would
/// refuse.
class MerchantOrderApi {
  /// Creates the API against [client].
  const MerchantOrderApi(this._client);

  final GoklayApiClient _client;

  /// The shop's orders. [live] asks for the board rather than the history.
  Future<ApiPage<MerchantOrderList>> list({
    required String merchantId,
    bool live = true,
    int limit = 20,
    int offset = 0,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/orders',
      query: <String, String>{
        if (live) 'live': 'true',
        'limit': '$limit',
        'offset': '$offset',
      },
    );
    return ApiPage<MerchantOrderList>.of(
      response,
      MerchantOrderList.fromJson(response.asObject),
    );
  }

  /// One order.
  Future<ApiPage<MerchantOrder>> order({
    required String merchantId,
    required String orderId,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/merchants/$merchantId/orders/$orderId',
    );
    return ApiPage<MerchantOrder>.of(
      response,
      MerchantOrder.fromJson(response.asObject),
    );
  }

  /// Takes the order.
  Future<MerchantOrder> accept({
    required String merchantId,
    required String orderId,
  }) => _transition(merchantId, orderId, 'accept');

  /// Refuses it. The reason is shown to the customer, so it is the shop's own
  /// words and not a code the app picks.
  Future<MerchantOrder> reject({
    required String merchantId,
    required String orderId,
    required String reason,
  }) => _transition(
    merchantId,
    orderId,
    'reject',
    body: <String, Object?>{'reason': reason},
  );

  /// Says the kitchen has started.
  Future<MerchantOrder> preparing({
    required String merchantId,
    required String orderId,
  }) => _transition(merchantId, orderId, 'preparing');

  /// Says it is ready for a rider.
  ///
  /// This is the transition that starts dispatch (P12): the offer round runs
  /// once here and asks the partners already on shift. Nothing about that is
  /// the app's business — it presses the button the server offered.
  Future<MerchantOrder> ready({
    required String merchantId,
    required String orderId,
  }) => _transition(merchantId, orderId, 'ready');

  Future<MerchantOrder> _transition(
    String merchantId,
    String orderId,
    String action, {
    Object? body,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/merchants/$merchantId/orders/$orderId/$action',
      body: body,
    );
    return MerchantOrder.fromJson(response.asObject);
  }
}
