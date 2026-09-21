import 'package:flutter/foundation.dart';

import '../api/json.dart';

/// Where a coordinate sits in Bangladesh's administrative hierarchy.
@immutable
class Area {
  /// Creates an area.
  const Area({
    required this.areaCode,
    required this.areaName,
    required this.districtCode,
    required this.divisionCode,
    required this.divisionName,
  });

  /// Reads the `Area` response.
  factory Area.fromJson(Map<String, Object?> json) => Area(
    areaCode: readString(json, 'area_code'),
    areaName: readString(json, 'area_name'),
    districtCode: readString(json, 'district_code'),
    divisionCode: readString(json, 'division_code'),
    divisionName: readString(json, 'division_name'),
  );

  /// The area code, such as `DHK-DHM`.
  final String areaCode;

  /// Its name, to show.
  final String areaName;

  /// The district.
  final String districtCode;

  /// One of the eight divisions. **D3**: nothing the customer does can widen a
  /// search past this boundary, and the app never tries to — it shows what
  /// discovery returns.
  final String divisionCode;

  /// The division's name, to show.
  final String divisionName;
}

/// The signed-in customer.
@immutable
class Profile {
  /// Creates a profile.
  const Profile({
    required this.userId,
    required this.name,
    required this.displayName,
    required this.email,
    required this.language,
  });

  /// Reads the `Profile` response.
  factory Profile.fromJson(Map<String, Object?> json) => Profile(
    userId: readString(json, 'user_id'),
    name: readString(json, 'name'),
    displayName: readString(json, 'display_name'),
    email: readString(json, 'email'),
    language: readString(json, 'language'),
  );

  /// The account's id.
  final String userId;

  /// The name the customer gave, which may be empty: ordering does not
  /// require one.
  final String name;

  /// **What to greet them with.** Never empty — the server decides the
  /// fallback so two clients do not invent two different greetings for the
  /// same nameless customer (2.9).
  final String displayName;

  /// Their email, or empty.
  final String email;

  /// `bn` or `en`.
  final String language;

  /// Whether they have told us their name.
  bool get hasName => name.isNotEmpty;
}

/// A delivery address, with the one line every surface shows.
@immutable
class Address {
  /// Creates an address.
  const Address({
    required this.id,
    required this.label,
    required this.recipientName,
    required this.recipientPhone,
    required this.line1,
    required this.line2,
    required this.instructions,
    required this.singleLine,
    required this.lat,
    required this.lng,
    required this.areaName,
    required this.isDefault,
  });

  /// Reads the `Address` object.
  factory Address.fromJson(Map<String, Object?> json) => Address(
    id: readString(json, 'id'),
    label: readString(json, 'label'),
    recipientName: readString(json, 'recipient_name'),
    recipientPhone: readString(json, 'recipient_phone'),
    line1: readString(json, 'line1'),
    line2: readString(json, 'line2'),
    instructions: readString(json, 'instructions'),
    singleLine: readString(json, 'single_line'),
    lat: readDouble(json, 'lat'),
    lng: readDouble(json, 'lng'),
    areaName: readString(json, 'area_name'),
    isDefault: readBool(json, 'is_default'),
  );

  /// The address's id, which the cart and the order reference.
  final String id;

  /// What the customer calls it — Home, Office.
  final String label;

  /// Whoever will open the door, who need not be the account holder.
  final String recipientName;

  /// A number for the rider.
  final String recipientPhone;

  /// The street line.
  final String line1;

  /// The second line, or empty.
  final String line2;

  /// What to tell the rider.
  final String instructions;

  /// **The address on one line, composed by the server.** The app, the
  /// receipt and the rider's screen all show this same string, because a
  /// client that joined the parts itself would eventually join them
  /// differently (2.9).
  final String singleLine;

  /// Where the rider goes.
  final double lat;

  /// Where the rider goes.
  final double lng;

  /// The area it resolved to.
  final String areaName;

  /// Whether this is the one checkout starts with.
  final bool isDefault;
}
