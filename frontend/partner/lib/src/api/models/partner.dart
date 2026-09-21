import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

/// The rider, as the server knows them.
///
/// Two fields are the whole of the app's behaviour and neither is computed
/// here. [availability] is `offline`, `available` or `busy` — and **`busy` is
/// not something a rider declares**: it is what being at the concurrent limit
/// is called, and it resolves itself when they finish a delivery. Letting the
/// app set it by hand would give a rider a way to stay in the pool while
/// refusing every offer. [preferenceLabel] is D4's distance choice, said in
/// the rider's own language by the server.
@immutable
class DeliveryPartner {
  /// Creates a partner.
  const DeliveryPartner({
    required this.id,
    required this.name,
    required this.phone,
    required this.vehicle,
    required this.availability,
    required this.availabilityLabel,
    required this.preference,
    required this.preferenceLabel,
    required this.lat,
    required this.lng,
    required this.carrying,
    required this.acceptancePercent,
  });

  /// Reads the `DeliveryPartner` response.
  factory DeliveryPartner.fromJson(Map<String, Object?> json) =>
      DeliveryPartner(
        id: readString(json, 'id'),
        name: readString(json, 'name'),
        phone: readString(json, 'phone'),
        vehicle: readString(json, 'vehicle'),
        availability: readString(json, 'availability'),
        availabilityLabel: readString(json, 'availability_label'),
        preference: readString(json, 'preference'),
        preferenceLabel: readString(json, 'preference_label'),
        lat: readDouble(json, 'lat'),
        lng: readDouble(json, 'lng'),
        carrying: readInt(json, 'carrying'),
        acceptancePercent: readInt(json, 'acceptance_percent'),
      );

  /// The partner's id.
  final String id;

  /// Their name.
  final String name;

  /// Their number.
  final String phone;

  /// What they ride.
  final String vehicle;

  /// `offline`, `available` or `busy`.
  final String availability;

  /// That state as a sentence, composed by the server.
  final String availabilityLabel;

  /// `short`, `long` or `any`. **D4.** `any` is the default: a partner who
  /// has not chosen has not chosen to exclude anything.
  final String preference;

  /// That choice as a sentence.
  final String preferenceLabel;

  /// Where they last reported being.
  final double lat;

  /// Where they last reported being.
  final double lng;

  /// How many live deliveries they hold.
  final int carrying;

  /// What share of offers they have taken. The server computes it.
  final int acceptancePercent;

  /// Offline.
  static const String offline = 'offline';

  /// On shift and taking offers.
  static const String available = 'available';

  /// At the concurrent limit. Never sent by the app.
  static const String busy = 'busy';

  /// Whether they are on shift at all.
  bool get isOnShift => availability != offline;

  /// Whether the server says they are at their limit — which is why the shift
  /// switch is shown but the reason for an empty feed comes from the feed.
  bool get isAtCapacity => availability == busy;
}
