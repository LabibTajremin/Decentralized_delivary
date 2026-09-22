import 'package:flutter/foundation.dart';
import 'package:goklay_core/goklay_core.dart';

import 'partner.dart';

/// One end of a delivery.
@immutable
class DeliveryPlace {
  /// Creates a place.
  const DeliveryPlace({
    required this.name,
    required this.phone,
    required this.singleLine,
    required this.lat,
    required this.lng,
  });

  /// Reads the `DeliveryPlace` object.
  factory DeliveryPlace.fromJson(Map<String, Object?> json) => DeliveryPlace(
    name: readString(json, 'name'),
    phone: readString(json, 'phone'),
    singleLine: readString(json, 'single_line'),
    lat: readDouble(json, 'lat'),
    lng: readDouble(json, 'lng'),
  );

  /// Who is there.
  final String name;

  /// A number to call.
  final String phone;

  /// The address on one line, composed by the server.
  final String singleLine;

  /// Where to go.
  final double lat;

  /// Where to go.
  final double lng;
}

/// One delivery, offered or held.
///
/// [band] is D4's distance band and [distance] is the pickup-to-door figure
/// already rendered as a string. The app never works out a distance: the
/// server has the coordinates, the ladder and the rounding, and a second
/// calculation here would disagree with the one that decided the job was
/// offered at all.
@immutable
class DeliveryJob {
  /// Creates a job.
  const DeliveryJob({
    required this.id,
    required this.orderId,
    required this.code,
    required this.status,
    required this.statusLabel,
    required this.live,
    required this.band,
    required this.bandLabel,
    required this.distance,
    required this.toPickup,
    required this.pickup,
    required this.destination,
    required this.reason,
    required this.secondsLeft,
  });

  /// Reads the `DeliveryJob` object.
  factory DeliveryJob.fromJson(Map<String, Object?> json) => DeliveryJob(
    id: readString(json, 'id'),
    orderId: readString(json, 'order_id'),
    code: readString(json, 'code'),
    status: readString(json, 'status'),
    statusLabel: readString(json, 'status_label'),
    live: readBool(json, 'live'),
    band: readString(json, 'band'),
    bandLabel: readString(json, 'band_label'),
    distance: readString(json, 'distance'),
    toPickup: readString(json, 'to_pickup'),
    pickup: DeliveryPlace.fromJson(readObject(json, 'pickup')),
    destination: DeliveryPlace.fromJson(readObject(json, 'destination')),
    reason: readString(json, 'reason'),
    secondsLeft: readInt(json, 'seconds_left'),
  );

  /// The job's id.
  final String id;

  /// The order behind it.
  final String orderId;

  /// The order's short code, so a rider and a shopkeeper say the same six
  /// characters to each other.
  final String code;

  /// `waiting`, `offered`, `assigned`, `collected`, `delivered`, `failed` or
  /// `cancelled`.
  final String status;

  /// That state as a sentence.
  final String statusLabel;

  /// Whether it is still going.
  final bool live;

  /// `short`, `long` or `beyond`.
  final String band;

  /// That band as a sentence.
  final String bandLabel;

  /// Pickup to door, rendered.
  final String distance;

  /// How far the rider is from the counter, rendered.
  final String toPickup;

  /// Where to collect.
  final DeliveryPlace pickup;

  /// Where to deliver.
  final DeliveryPlace destination;

  /// Why it failed, where it did.
  final String reason;

  /// How long an offer has left. The server's number; the screen counts it
  /// down for the look of the thing and re-reads the feed rather than
  /// deciding for itself that an offer has lapsed.
  final int secondsLeft;

  /// A job that has been offered and not yet answered.
  static const String offered = 'offered';

  /// A job this rider holds but has not collected.
  static const String assigned = 'assigned';

  /// A job in the rider's bag.
  static const String collected = 'collected';

  /// Whether the rider may accept or decline it.
  bool get isOffer => status == offered;

  /// Whether the next thing to do is collect it.
  bool get awaitsCollection => status == assigned;

  /// Whether the next thing to do is hand it over.
  bool get awaitsDelivery => status == collected;
}

/// A page of the rider's own deliveries.
@immutable
class DeliveryJobList {
  /// Creates a page.
  const DeliveryJobList({required this.jobs, required this.total});

  /// Reads the `DeliveryJobList` response.
  factory DeliveryJobList.fromJson(Map<String, Object?> json) =>
      DeliveryJobList(
        jobs: readList(json, 'jobs', DeliveryJob.fromJson),
        total: readInt(json, 'total'),
      );

  /// This page's jobs.
  final List<DeliveryJob> jobs;

  /// How many matched before paging.
  final int total;

  /// Whether the rider holds nothing.
  bool get isEmpty => jobs.isEmpty;
}

/// What the rider should be looking at right now (ALG-08).
///
/// The feed is bounded, nearest-pickup first, and already filtered by the
/// rider's own distance choice. A client that filtered a global list by its
/// own radius would be deciding visibility — which 2.9 forbids, and which
/// would need a copy of `dispatch.partner_radius` to do at all.
///
/// An empty feed carries a [reason] and a [notice]. A rider who forgot to go
/// on shift is told that, rather than shown nothing and left to wonder.
@immutable
class PartnerFeed {
  /// Creates a feed.
  const PartnerFeed({
    required this.partner,
    required this.jobs,
    required this.reason,
    required this.notice,
  });

  /// Reads the `PartnerFeed` response.
  factory PartnerFeed.fromJson(Map<String, Object?> json) => PartnerFeed(
    partner: DeliveryPartner.fromJson(readObject(json, 'partner')),
    jobs: readList(json, 'jobs', DeliveryJob.fromJson),
    reason: readOptionalString(json, 'reason'),
    notice: readString(json, 'notice'),
  );

  /// The rider this feed is for.
  final DeliveryPartner partner;

  /// What they are being offered.
  final List<DeliveryJob> jobs;

  /// `offline`, `at_capacity` or `nothing_nearby` when the list is empty.
  /// A code to branch on, never to print.
  final String? reason;

  /// The sentence to show, composed by the server.
  final String notice;

  /// Whether there is anything to take.
  bool get isEmpty => jobs.isEmpty;
}
