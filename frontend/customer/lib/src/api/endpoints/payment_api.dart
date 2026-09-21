import 'package:goklay_core/goklay_core.dart';

import '../api_page.dart';
import '../models/payment.dart';

/// Paying for an order that was placed `online`.
class PaymentApi {
  /// Creates the API against [client].
  const PaymentApi(this._client);

  final GoklayApiClient _client;

  /// Starts, or resumes, a checkout for an order.
  ///
  /// Resuming is the normal case rather than an edge one: a customer who
  /// closed the payment page comes back to a `pending_payment` order, and the
  /// server hands back the attempt it already has rather than charging twice.
  Future<Checkout> checkout(String orderId) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/payments/checkout',
      body: <String, Object?>{'order_id': orderId},
    );
    return Checkout.fromJson(response.asObject);
  }

  /// The payment's current state.
  Future<ApiPage<Payment>> payment(String orderId) async {
    final ApiResponse response = await _client.get('/v1/payments/$orderId');
    return ApiPage<Payment>.of(response, Payment.fromJson(response.asObject));
  }
}
