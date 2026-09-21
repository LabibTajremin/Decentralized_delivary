import 'package:flutter/foundation.dart';

import '../json.dart';

/// One notification the server sent, or tried to.
///
/// Both the title and the body are composed by the server, in the customer's
/// language, which is why nothing here is assembled from an order's fields
/// (2.9). The app lists what it is given.
@immutable
class AppNotification {
  /// Creates a notification.
  const AppNotification({
    required this.id,
    required this.title,
    required this.body,
    required this.channel,
    required this.status,
    required this.createdAt,
  });

  /// Reads the `Notification` object.
  factory AppNotification.fromJson(Map<String, Object?> json) =>
      AppNotification(
        id: readString(json, 'id'),
        title: readString(json, 'title'),
        body: readString(json, 'body'),
        channel: readString(json, 'channel'),
        status: readString(json, 'status'),
        createdAt: DateTime.tryParse(readString(json, 'created_at')),
      );

  /// The notification's id.
  final String id;

  /// Its heading.
  final String title;

  /// Its text.
  final String body;

  /// `push`, `sms` or `none`.
  final String channel;

  /// `sent` or `failed`.
  final String status;

  /// When it was raised.
  final DateTime? createdAt;

  /// Whether it never reached the device, so the list can mark it rather than
  /// showing it as though the customer had seen it.
  bool get didFail => status == 'failed';
}
