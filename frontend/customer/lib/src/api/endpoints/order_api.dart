import 'package:goklay_core/goklay_core.dart';

import '../models/order.dart';

/// A page of the customer's orders.
class OrderList {
  /// Creates a page.
  const OrderList({required this.orders, required this.total});

  /// Reads the `OrderList` response.
  factory OrderList.fromJson(Map<String, Object?> json) {
    final Object? raw = json['orders'];
    return OrderList(
      orders: <Order>[
        if (raw is List<Object?>)
          for (final Object? entry in raw)
            if (entry is Map<String, Object?>) Order.fromJson(entry),
      ],
      total: (json['total'] as num?)?.toInt() ?? 0,
    );
  }

  /// This page's orders.
  final List<Order> orders;

  /// How many matched before paging.
  final int total;

  /// Whether the customer has never ordered.
  bool get isEmpty => orders.isEmpty;
}

/// Placing, listing and cancelling orders.
class OrderApi {
  /// Creates the API against [client].
  const OrderApi(this._client);

  final GoklayApiClient _client;

  /// Places the cart as an order.
  ///
  /// [idempotencyKey] is not optional in practice and is why a retried tap
  /// cannot place two orders: the app sends the same key for the same attempt
  /// and the server returns the order it already made.
  Future<Order> place({
    required String addressId,
    required String paymentMethod,
    required String idempotencyKey,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/orders',
      body: <String, Object?>{
        'address_id': addressId,
        'payment_method': paymentMethod,
        'idempotency_key': idempotencyKey,
      },
    );
    return Order.fromJson(response.asObject);
  }

  /// The customer's orders. [live] asks for the current-orders tab.
  Future<ApiPage<OrderList>> list({
    bool live = false,
    int limit = 20,
    int offset = 0,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/orders',
      query: <String, String>{
        if (live) 'live': 'true',
        'limit': '$limit',
        'offset': '$offset',
      },
    );
    return ApiPage<OrderList>.of(
      response,
      OrderList.fromJson(response.asObject),
    );
  }

  /// One order.
  Future<ApiPage<Order>> order(String orderId) async {
    final ApiResponse response = await _client.get('/v1/orders/$orderId');
    return ApiPage<Order>.of(response, Order.fromJson(response.asObject));
  }

  /// Whether the order can still be cancelled, and for how long.
  ///
  /// Read separately from the order because the answer changes with the clock
  /// while the screen is open, and the screen re-reads it rather than counting
  /// its own seconds down to zero and deciding the window has shut.
  Future<ApiPage<OrderCancellation>> cancellation(String orderId) async {
    final ApiResponse response = await _client.get(
      '/v1/orders/$orderId/cancellation',
    );
    return ApiPage<OrderCancellation>.of(
      response,
      OrderCancellation.fromJson(response.asObject),
    );
  }

  /// Cancels an order.
  Future<Order> cancel({required String orderId, String reason = ''}) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/orders/$orderId/cancel',
      body: <String, Object?>{'reason': reason},
    );
    return Order.fromJson(response.asObject);
  }
}
