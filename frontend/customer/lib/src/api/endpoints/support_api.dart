import 'package:goklay_core/goklay_core.dart';

import '../models/support.dart';

/// Reviews, ratings and support tickets.
class SupportApi {
  /// Creates the API against [client].
  const SupportApi(this._client);

  final GoklayApiClient _client;

  /// Leaves a review. The server checks the caller was actually on the order
  /// and that the subject was actually part of it, so the app can offer the
  /// form without deciding who is entitled to fill it in.
  Future<Review> submitReview({
    required String orderId,
    required ReviewSubject subject,
    required String subjectId,
    required int rating,
    String comment = '',
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/reviews',
      body: <String, Object?>{
        'order_id': orderId,
        'subject': subject.wireValue,
        'subject_id': subjectId,
        'rating': rating,
        'comment': comment,
      },
    );
    return Review.fromJson(response.asObject);
  }

  /// A subject's reviews.
  Future<ApiPage<List<Review>>> reviews({
    required ReviewSubject subject,
    required String subjectId,
    int limit = 20,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/reviews',
      query: <String, String>{
        'subject': subject.wireValue,
        'subject_id': subjectId,
        'limit': '$limit',
      },
    );
    return ApiPage<List<Review>>.of(
      response,
      _list(response.asObject['reviews'], Review.fromJson),
    );
  }

  /// A subject's aggregate rating.
  Future<ApiPage<Rating>> rating({
    required ReviewSubject subject,
    required String subjectId,
  }) async {
    final ApiResponse response = await _client.get(
      '/v1/ratings',
      query: <String, String>{
        'subject': subject.wireValue,
        'subject_id': subjectId,
      },
    );
    return ApiPage<Rating>.of(response, Rating.fromJson(response.asObject));
  }

  /// Raises a ticket against an order.
  Future<SupportTicket> raiseTicket({
    required String orderId,
    required String subject,
  }) async {
    final ApiResponse response = await _client.send(
      'POST',
      '/v1/support/tickets',
      body: <String, Object?>{'order_id': orderId, 'subject': subject},
    );
    return SupportTicket.fromJson(response.asObject);
  }

  /// The customer's own tickets.
  Future<ApiPage<List<SupportTicket>>> tickets() async {
    final ApiResponse response = await _client.get('/v1/me/support/tickets');
    return ApiPage<List<SupportTicket>>.of(
      response,
      _list(response.asObject['tickets'], SupportTicket.fromJson),
    );
  }

  static List<T> _list<T>(
    Object? raw,
    T Function(Map<String, Object?>) parse,
  ) {
    if (raw is! List<Object?>) {
      return <T>[];
    }
    return <T>[
      for (final Object? entry in raw)
        if (entry is Map<String, Object?>) parse(entry),
    ];
  }
}
