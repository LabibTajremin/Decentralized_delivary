import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_merchant/src/api/models/catalogue.dart';
import 'package:goklay_merchant/src/api/models/merchant.dart';
import 'package:goklay_merchant/src/api/models/order.dart';

import 'support/fixtures.dart';

void main() {
  group('merchant', () {
    test('parses the whole record, hours and documents included', () {
      final Merchant shop = Merchant.fromJson(merchantJson());
      expect(shop.id, 'mch-1');
      expect(shop.singleLine, '১২/এ, মিরপুর রোড, ধানমন্ডি, ঢাকা');
      expect(shop.divisionCode, 'DHA');
      expect(shop.hours['0'], <String>['09:00-22:00']);
      expect(shop.documents.single.number, 'TL-99');
      expect(shop.requiredDocuments, hasLength(2));
      expect(shop.holiday, isNull);
      expect(shop.hasReviewNote, isFalse);
    });

    test('the status flags are read, never derived from each other', () {
      expect(Merchant.fromJson(merchantJson()).isApproved, isTrue);
      expect(
        Merchant.fromJson(merchantJson(status: 'draft')).isDraft,
        isTrue,
      );
      expect(
        Merchant.fromJson(
          merchantJson(status: 'pending_review'),
        ).isAwaitingReview,
        isTrue,
      );
    });

    test('a holiday takes the shop off the list, and says so', () {
      final Merchant shop = Merchant.fromJson(merchantJson(holiday: true));
      expect(shop.holiday!.reason, 'ঈদের ছুটি');
      expect(shop.holiday!.until, isNotNull);
      expect(shop.isListed, isFalse);
      expect(shop.isOpenNow, isFalse);
    });

    test('what is still wanted is the server\'s list, not a subtraction', () {
      final Merchant shop = Merchant.fromJson(
        merchantJson(missing: <String>['food_licence'], canSubmit: false),
      );
      expect(shop.missingDocuments, <String>['food_licence']);
      expect(shop.canSubmit, isFalse);
      expect(shop.documents, hasLength(1));
    });

    test('a rejected shop carries the reason it was given', () {
      final Merchant shop = Merchant.fromJson(
        merchantJson(status: 'rejected', reviewNote: 'কাগজ পড়া যায়নি'),
      );
      expect(shop.hasReviewNote, isTrue);
      expect(shop.reviewNote, 'কাগজ পড়া যায়নি');
    });

    test('a list of anything but strings is dropped rather than crashing', () {
      final Map<String, Object?> body = merchantJson()
        ..['next_statuses'] = <Object?>['pending_review', 7]
        ..['hours'] = <String, Object?>{'0': 'not a list'};
      final Merchant shop = Merchant.fromJson(body);
      expect(shop.nextStatuses, <String>['pending_review']);
      expect(shop.hours['0'], isEmpty);
    });
  });

  group('registration requirements', () {
    test('the types and their documents come from the server', () {
      final RegistrationRequirements needed =
          RegistrationRequirements.fromJson(requirementsJson());
      expect(needed.types, <String>['restaurant', 'grocery', 'pharmacy']);
      expect(needed.byType['pharmacy'], contains('drug_licence'));
    });

    test('a malformed body is an empty map, not a throw', () {
      expect(
        RegistrationRequirements.fromJson(<String, Object?>{
          'types': 'nope',
        }).types,
        isEmpty,
      );
    });
  });

  group('catalogue capabilities', () {
    test('a restaurant has add-ons and bundles but no shelf', () {
      final CatalogueCapabilities capabilities =
          CatalogueCapabilities.fromJson(capabilitiesJson());
      expect(capabilities.addons, isTrue);
      expect(capabilities.combos, isTrue);
      expect(capabilities.tracksStock, isFalse);
      expect(capabilities.requiresUnit, isFalse);
      expect(capabilities.units, isEmpty);
    });

    test('a grocery counts a shelf and needs a unit', () {
      final CatalogueCapabilities capabilities = CatalogueCapabilities.fromJson(
        capabilitiesJson(type: 'grocery'),
      );
      expect(capabilities.tracksStock, isTrue);
      expect(capabilities.requiresUnit, isTrue);
      expect(capabilities.units, contains('kg'));
      expect(capabilities.prescriptions, isFalse);
    });

    test('only a pharmacy has prescriptions', () {
      expect(
        CatalogueCapabilities.fromJson(
          capabilitiesJson(type: 'pharmacy'),
        ).prescriptions,
        isTrue,
      );
    });

    test('a units field that is not a list is an empty list', () {
      final Map<String, Object?> body = capabilitiesJson()..['units'] = 7;
      expect(CatalogueCapabilities.fromJson(body).units, isEmpty);
    });
  });

  group('owner catalogue', () {
    test('a section carries the shop\'s own order and switch', () {
      final OwnerCategory category = OwnerCategory.fromJson(categoryJson());
      expect(category.sortOrder, 1);
      expect(category.active, isTrue);
    });

    test('an item reports orderable separately from its own switch', () {
      final OwnerItem item = OwnerItem.fromJson(
        itemJson(active: true, orderable: false),
      );
      expect(item.active, isTrue);
      expect(item.orderable, isFalse);
      expect(item.unavailableReason, 'out_of_stock');
    });

    test('a shelf count of null is not the same as zero', () {
      expect(OwnerItem.fromJson(itemJson()).stockQuantity, isNull);
      expect(
        OwnerItem.fromJson(
          itemJson(stockTracked: true, stock: 0),
        ).stockQuantity,
        0,
      );
    });

    test('a combo lists its members by name', () {
      final OwnerCombo combo = OwnerCombo.fromJson(comboJson());
      expect(combo.lineNames, <String>['কাচ্চি']);
      expect(combo.orderable, isTrue);
      final Map<String, Object?> broken = comboJson()..['lines'] = 'nope';
      expect(OwnerCombo.fromJson(broken).lineNames, isEmpty);
    });
  });

  group('the order board', () {
    test('next_actions is the whole of what a shop may do', () {
      final MerchantOrder order = MerchantOrder.fromJson(orderJson());
      expect(order.allows(MerchantOrder.accepted), isTrue);
      expect(order.allows(MerchantOrder.rejected), isTrue);
      expect(order.allows(MerchantOrder.ready), isFalse);
      expect(order.needsAnswer, isTrue);
    });

    test('an order with no moves left offers nothing', () {
      final MerchantOrder order = MerchantOrder.fromJson(
        orderJson(
          status: 'delivered',
          live: false,
          nextActions: const <String>[],
        ),
      );
      expect(order.nextActions, isEmpty);
      expect(order.needsAnswer, isFalse);
      expect(order.live, isFalse);
    });

    test('the lines carry the note and the options the kitchen needs', () {
      final MerchantOrder order = MerchantOrder.fromJson(orderJson());
      final OrderLine line = order.lines.single;
      expect(line.quantity, 2);
      expect(line.note, 'ঝাল কম');
      expect(line.options, <String>['বড়']);
      expect(line.lineTotal.display, '৳ ৬৪০');
      final Map<String, Object?> broken = orderJson();
      (broken['lines']! as List<Object?>)[0] =
          <String, Object?>{'name': 'x', 'options': 'nope'};
      expect(MerchantOrder.fromJson(broken).lines.single.options, isEmpty);
    });

    test('the receipt, the destination and the timeline parse whole', () {
      final MerchantOrder order = MerchantOrder.fromJson(orderJson());
      expect(order.receipt.single.label, 'সাবটোটাল');
      expect(order.destination.singleLine, 'রোড ৫, ধানমন্ডি, ঢাকা');
      expect(order.destination.phone, '01712345678');
      expect(order.events.single.label, 'অর্ডার হয়েছে');
      expect(order.events.single.reason, isEmpty);
      expect(order.placedAt, isNotNull);
      expect(order.paymentMethod, 'cash');
      expect(order.count, 2);
    });

    test('next_actions ignores anything that is not a string', () {
      final Map<String, Object?> body = orderJson()
        ..['next_actions'] = <Object?>['accepted', 7];
      expect(MerchantOrder.fromJson(body).nextActions, <String>['accepted']);
      final Map<String, Object?> missing = orderJson()
        ..remove('next_actions');
      expect(MerchantOrder.fromJson(missing).nextActions, isEmpty);
    });

    test('a page of orders reports emptiness', () {
      expect(
        MerchantOrderList.fromJson(orderListJson()).isEmpty,
        isFalse,
      );
      expect(
        MerchantOrderList.fromJson(
          orderListJson(orders: <Map<String, Object?>>[]),
        ).isEmpty,
        isTrue,
      );
      final Map<String, Object?> broken = <String, Object?>{
        'orders': <Object?>[orderJson(), 'not an order'],
        'total': 1,
      };
      expect(MerchantOrderList.fromJson(broken).orders, hasLength(1));
      expect(
        MerchantOrderList.fromJson(<String, Object?>{}).orders,
        isEmpty,
      );
    });
  });
}
