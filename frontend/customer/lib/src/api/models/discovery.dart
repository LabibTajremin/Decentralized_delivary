import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';


/// The delivery fee quoted on a shop card.
///
/// [display] is what the card shows. [expanded] says the quote includes D2's
/// surcharge for reaching past the base radius — the app renders that fact,
/// it does not work out when it applies.
@immutable
class DeliveryFee {
  /// Creates a quoted fee.
  const DeliveryFee({required this.money, required this.expanded});

  /// Reads the `DeliveryFee` object.
  factory DeliveryFee.fromJson(Map<String, Object?> json) => DeliveryFee(
    money: Money.fromJson(json),
    expanded: readBool(json, 'expanded'),
  );

  /// The fee, minor units and display string together.
  final Money money;

  /// Whether this quote crossed the base radius and carries a surcharge.
  final bool expanded;
}

/// One shop on the home screen.
///
/// Every field is final. `distance` is a rendered string, `open_status` is a
/// sentence, and whether the shop appears at all was decided by discovery
/// against D1, D2 and D3. There is nothing here to compute (2.9).
@immutable
class DiscoveryMerchant {
  /// Creates a shop card.
  const DiscoveryMerchant({
    required this.id,
    required this.name,
    required this.type,
    required this.logoUrl,
    required this.areaName,
    required this.distance,
    required this.isOpenNow,
    required this.openStatus,
    required this.delivery,
  });

  /// Reads the `DiscoveryMerchant` object.
  factory DiscoveryMerchant.fromJson(Map<String, Object?> json) =>
      DiscoveryMerchant(
        id: readString(json, 'id'),
        name: readString(json, 'name'),
        type: readString(json, 'type'),
        logoUrl: readString(json, 'logo_url'),
        areaName: readString(json, 'area_name'),
        distance: readString(json, 'distance'),
        isOpenNow: readBool(json, 'is_open_now'),
        openStatus: readString(json, 'open_status'),
        delivery: DeliveryFee.fromJson(readObject(json, 'delivery')),
      );

  /// The shop's id, for the menu request.
  final String id;

  /// Its name.
  final String name;

  /// `restaurant`, `grocery` or `pharmacy` — which chip filters it.
  final String type;

  /// A pre-sized image URL. The app never resizes an image (2.9).
  final String logoUrl;

  /// The area it is in, as a name to show.
  final String areaName;

  /// How far away, already formatted in the caller's language.
  final String distance;

  /// Whether it is open, for the enabled state of the card.
  final bool isOpenNow;

  /// The sentence to show beside the name — "Opens at 11am" or similar.
  final String openStatus;

  /// The quoted delivery fee.
  final DeliveryFee delivery;
}

/// How far the current search reached, and what widening would do.
///
/// D2 charges for expansion, so the decision belongs to the customer — which
/// means the app has to show the offer, and the server has to have already
/// decided whether there is one ([canExpand]) and what it would cost.
@immutable
class DiscoveryExpansion {
  /// Creates an expansion state.
  const DiscoveryExpansion({
    required this.level,
    required this.radius,
    required this.stage,
    required this.canExpand,
    required this.offered,
    required this.atCeiling,
    required this.nextLevel,
    required this.nextRadius,
  });

  /// Reads the `DiscoveryExpansion` object.
  factory DiscoveryExpansion.fromJson(Map<String, Object?> json) =>
      DiscoveryExpansion(
        level: readInt(json, 'level'),
        radius: readString(json, 'radius'),
        stage: readString(json, 'stage'),
        canExpand: readBool(json, 'can_expand'),
        offered: readBool(json, 'offered'),
        atCeiling: readBool(json, 'at_ceiling'),
        nextLevel: readInt(json, 'next_level'),
        nextRadius: readString(json, 'next_radius'),
      );

  /// Which rung of the ladder this search used.
  final int level;

  /// The radius searched, formatted.
  final String radius;

  /// `base`, `expanded` or `ceiling`.
  final String stage;

  /// Whether widening is possible at all.
  final bool canExpand;

  /// Whether the server is suggesting it, because results are thin.
  final bool offered;

  /// Whether this is as far as the division allows (D3). When true there is
  /// nothing to offer and the screen must not imply otherwise.
  final bool atCeiling;

  /// The rung widening would move to.
  final int nextLevel;

  /// What that rung's radius would be, formatted.
  final String nextRadius;
}

/// A whole home-screen search.
@immutable
class DiscoverySearch {
  /// Creates a search result.
  const DiscoverySearch({
    required this.areaName,
    required this.notice,
    required this.total,
    required this.expansion,
    required this.merchants,
  });

  /// Reads the `DiscoverySearch` response.
  factory DiscoverySearch.fromJson(Map<String, Object?> json) =>
      DiscoverySearch(
        areaName: readString(json, 'area_name'),
        notice: readString(json, 'notice'),
        total: readInt(json, 'total'),
        expansion: DiscoveryExpansion.fromJson(readObject(json, 'expansion')),
        merchants: readList(json, 'merchants', DiscoveryMerchant.fromJson),
      );

  /// Where the server decided the customer is.
  final String areaName;

  /// The one line above the list, composed by the server: nothing found,
  /// expansion offered, or the ceiling reached. Four genuinely different
  /// messages, which is why the app does not assemble it from parts.
  final String notice;

  /// How many shops matched before paging.
  final int total;

  /// The expansion state.
  final DiscoveryExpansion expansion;

  /// The shops to show.
  final List<DiscoveryMerchant> merchants;
}
