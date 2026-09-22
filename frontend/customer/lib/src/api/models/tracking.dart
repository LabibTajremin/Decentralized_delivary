import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';


/// The rider carrying an order, present only while one is.
@immutable
class TrackingPartner {
  /// Creates a partner.
  const TrackingPartner({
    required this.id,
    required this.name,
    required this.phone,
    required this.vehicle,
    required this.lat,
    required this.lng,
  });

  /// Reads the `TrackingPartner` object.
  factory TrackingPartner.fromJson(Map<String, Object?> json) =>
      TrackingPartner(
        id: readString(json, 'id'),
        name: readString(json, 'name'),
        phone: readString(json, 'phone'),
        vehicle: readString(json, 'vehicle'),
        lat: readDouble(json, 'lat'),
        lng: readDouble(json, 'lng'),
      );

  /// The rider's id.
  final String id;

  /// Their name.
  final String name;

  /// A number to call them on.
  final String phone;

  /// What they are riding.
  final String vehicle;

  /// Where they last reported being.
  final double lat;

  /// Where they last reported being.
  final double lng;
}

/// One frame of a live delivery.
///
/// The tracking endpoint is a server-sent-event stream, so [live] is how the
/// screen knows the stream is finished rather than stalled — a client that
/// decided for itself which statuses are terminal would go stale the first
/// time the order state machine gained a state.
@immutable
class TrackingSnapshot {
  /// Creates a snapshot.
  const TrackingSnapshot({
    required this.orderId,
    required this.status,
    required this.statusLabel,
    required this.live,
    required this.partner,
  });

  /// Reads the `TrackingSnapshot` frame.
  factory TrackingSnapshot.fromJson(Map<String, Object?> json) {
    final Map<String, Object?> partner = readObject(json, 'partner');
    return TrackingSnapshot(
      orderId: readString(json, 'order_id'),
      status: readString(json, 'status'),
      statusLabel: readString(json, 'status_label'),
      live: readBool(json, 'live'),
      partner: partner.isEmpty ? null : TrackingPartner.fromJson(partner),
    );
  }

  /// Which order this is about.
  final String orderId;

  /// Its state, as a code.
  final String status;

  /// Its state as a sentence, composed by the server.
  final String statusLabel;

  /// False on the last frame this stream will ever send.
  final bool live;

  /// The rider, or null when nobody is carrying it yet.
  final TrackingPartner? partner;

  /// Whether there is a rider to show on the map.
  bool get hasPartner => partner != null;
}
