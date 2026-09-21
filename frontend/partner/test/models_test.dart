import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_partner/src/api/models/job.dart';
import 'package:goklay_partner/src/api/models/ledger.dart';
import 'package:goklay_partner/src/api/models/partner.dart';

import 'support/fixtures.dart';

void main() {
  group('the partner', () {
    test('parses the whole record', () {
      final DeliveryPartner partner = DeliveryPartner.fromJson(partnerJson());
      expect(partner.id, 'ptn-1');
      expect(partner.vehicle, 'bike');
      expect(partner.acceptancePercent, 82);
      expect(partner.lat, 23.75);
      expect(partner.carrying, 0);
    });

    test('the shift state is read, never inferred', () {
      expect(DeliveryPartner.fromJson(partnerJson()).isOnShift, isTrue);
      expect(
        DeliveryPartner.fromJson(
          partnerJson(availability: 'offline'),
        ).isOnShift,
        isFalse,
      );
    });

    test('busy is a state the server sets, and the app only reads', () {
      final DeliveryPartner busy = DeliveryPartner.fromJson(
        partnerJson(availability: 'busy', carrying: 2),
      );
      expect(busy.isAtCapacity, isTrue);
      expect(busy.isOnShift, isTrue);
      expect(busy.availabilityLabel, 'আপনি এখন ব্যস্ত');
    });

    test('D4\'s choice arrives with the sentence that explains it', () {
      final DeliveryPartner picky = DeliveryPartner.fromJson(
        partnerJson(preference: 'short'),
      );
      expect(picky.preference, 'short');
      expect(picky.preferenceLabel, 'কাছের ডেলিভারি');
    });
  });

  group('a job', () {
    test('the distances are rendered strings, not numbers to format', () {
      final DeliveryJob job = DeliveryJob.fromJson(jobJson());
      expect(job.distance, '৩.২ কিমি');
      expect(job.toPickup, '৪০০ মিটার');
      expect(job.bandLabel, 'কাছের');
      expect(job.secondsLeft, 45);
      expect(job.code, 'GK-7F3K');
    });

    test('the two ends carry a name, an address and a number', () {
      final DeliveryJob job = DeliveryJob.fromJson(jobJson());
      expect(job.pickup.name, 'নূরজাহান হোটেল');
      expect(job.pickup.phone, '01799999999');
      expect(job.destination.singleLine, 'রোড ৫, ধানমন্ডি, ঢাকা');
      expect(job.destination.lat, 23.7461);
    });

    test('exactly one of the three next-steps is true at a time', () {
      expect(DeliveryJob.fromJson(jobJson()).isOffer, isTrue);
      final DeliveryJob assigned = DeliveryJob.fromJson(
        jobJson(status: 'assigned'),
      );
      expect(assigned.awaitsCollection, isTrue);
      expect(assigned.isOffer, isFalse);
      final DeliveryJob collected = DeliveryJob.fromJson(
        jobJson(status: 'collected'),
      );
      expect(collected.awaitsDelivery, isTrue);
      expect(collected.awaitsCollection, isFalse);
    });

    test('a failed job carries the rider\'s reason', () {
      final DeliveryJob failed = DeliveryJob.fromJson(
        jobJson(status: 'failed', live: false, reason: 'দরজা খোলেনি'),
      );
      expect(failed.reason, 'দরজা খোলেনি');
      expect(failed.live, isFalse);
    });

    test('a page of jobs reports emptiness', () {
      expect(DeliveryJobList.fromJson(jobListJson()).isEmpty, isFalse);
      expect(
        DeliveryJobList.fromJson(
          jobListJson(jobs: <Map<String, Object?>>[]),
        ).isEmpty,
        isTrue,
      );
      expect(DeliveryJobList.fromJson(<String, Object?>{}).total, 0);
    });
  });

  group('the feed', () {
    test('a feed with work carries no reason', () {
      final PartnerFeed feed = PartnerFeed.fromJson(feedJson());
      expect(feed.isEmpty, isFalse);
      expect(feed.reason, isNull);
      expect(feed.partner.isOnShift, isTrue);
    });

    test('an empty feed says why, in a code and a sentence', () {
      final PartnerFeed feed = PartnerFeed.fromJson(
        feedJson(
          jobs: <Map<String, Object?>>[],
          availability: 'offline',
          reason: 'offline',
          notice: 'আপনি অফলাইন আছেন',
        ),
      );
      expect(feed.isEmpty, isTrue);
      expect(feed.reason, 'offline');
      expect(feed.notice, 'আপনি অফলাইন আছেন');
    });
  });

  group('the ledger', () {
    test('both totals are the server\'s, and the held list backs them', () {
      final Ledger ledger = Ledger.fromJson(ledgerJson());
      expect(ledger.outstanding.display, '৳ ৭০০');
      expect(ledger.remitted.display, '৳ ১,২০০');
      expect(ledger.held.single.orderId, 'ord-1');
      expect(ledger.held.single.isHeld, isTrue);
      expect(ledger.isSettled, isFalse);
    });

    test('a settled rider has nothing to hand over', () {
      final Ledger ledger = Ledger.fromJson(ledgerJson(settled: true));
      expect(ledger.isSettled, isTrue);
      expect(ledger.outstanding.minor, 0);
    });

    test('a remitted collection is no longer held', () {
      final Collection collection = Collection.fromJson(<String, Object?>{
        'id': 'col-2',
        'order_id': 'ord-2',
        'amount': money(1000, '৳ ১০'),
        'status': 'remitted',
        'status_label': 'জমা দেওয়া হয়েছে',
      });
      expect(collection.isHeld, isFalse);
      expect(collection.statusLabel, 'জমা দেওয়া হয়েছে');
    });
  });
}
