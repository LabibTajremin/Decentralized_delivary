import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/json.dart';
import 'package:goklay_customer/src/api/models/account.dart';
import 'package:goklay_customer/src/api/models/auth.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/api/models/catalogue.dart';
import 'package:goklay_customer/src/api/models/discovery.dart';
import 'package:goklay_customer/src/api/models/notification.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/api/models/payment.dart';
import 'package:goklay_customer/src/api/models/support.dart';
import 'package:goklay_customer/src/api/models/tracking.dart';

import 'support/fixtures.dart';

void main() {
  group('json readers', () {
    const Map<String, Object?> full = <String, Object?>{
      'text': 'hello',
      'blank': '',
      'number': 7,
      'decimal': 1.5,
      'wholeAsDouble': 3.0,
      'flag': true,
      'object': <String, Object?>{'a': 1},
      'list': <Object?>[
        <String, Object?>{'a': 1},
        'not an object',
      ],
      'notAList': 'nope',
      'notAnObject': 'nope',
    };

    test('read the values that are there', () {
      expect(readString(full, 'text'), 'hello');
      expect(readOptionalString(full, 'text'), 'hello');
      expect(readInt(full, 'number'), 7);
      expect(readInt(full, 'wholeAsDouble'), 3);
      expect(readDouble(full, 'decimal'), 1.5);
      expect(readBool(full, 'flag'), isTrue);
      expect(readObject(full, 'object'), <String, Object?>{'a': 1});
    });

    test('a missing or blank field has a defined answer, never a throw', () {
      expect(readString(full, 'absent'), '');
      expect(readOptionalString(full, 'absent'), isNull);
      expect(readOptionalString(full, 'blank'), isNull);
      expect(readInt(full, 'absent'), 0);
      expect(readDouble(full, 'absent'), 0);
      expect(readObject(full, 'notAnObject'), isEmpty);
    });

    test('a flag defaults to false, because every flag is a capability', () {
      expect(readBool(full, 'absent'), isFalse);
    });

    test('a list skips entries that are not objects', () {
      expect(
        readList(full, 'list', (Map<String, Object?> j) => readInt(j, 'a')),
        <int>[1],
      );
      expect(
        readList(full, 'notAList', (Map<String, Object?> j) => j),
        isEmpty,
      );
    });
  });

  group('auth', () {
    test('a challenge carries the server\'s two durations', () {
      final OtpChallenge challenge = OtpChallenge.fromJson(<String, Object?>{
        'expires_in': 300,
        'resend_after': 720,
      });
      expect(challenge.expiresIn, const Duration(seconds: 300));
      expect(challenge.resendAfter, const Duration(seconds: 720));
    });

    test('only a customer token may use the customer app', () {
      expect(AuthResult.fromJson(tokenPairJson()).isCustomer, isTrue);
      expect(
        AuthResult.fromJson(tokenPairJson(role: 'partner')).isCustomer,
        isFalse,
      );
    });

    test('a device session carries a label and a time, never a token', () {
      final DeviceSession session = DeviceSession.fromJson(<String, Object?>{
        'session_id': 'ses-1',
        'device': 'Pixel 8',
        'last_seen_at': '2026-09-21T10:00:00Z',
        'current': true,
      });
      expect(session.device, 'Pixel 8');
      expect(session.isCurrent, isTrue);
      expect(session.lastSeenAt, isNotNull);
    });
  });

  group('discovery', () {
    test('a shop card is rendered strings, not numbers to format', () {
      final DiscoverySearch search = DiscoverySearch.fromJson(searchJson());
      final DiscoveryMerchant merchant = search.merchants.single;
      expect(merchant.distance, '১.২ কিমি');
      expect(merchant.openStatus, 'খোলা আছে');
      expect(merchant.delivery.money.display, '৳ ৬০');
      expect(merchant.delivery.expanded, isFalse);
      expect(search.expansion.nextLevel, 1);
    });
  });

  group('catalogue', () {
    test('a group with one choice is a radio, more is a checkbox', () {
      expect(OptionGroup.fromJson(groupJson()).isMultiSelect, isFalse);
      expect(OptionGroup.fromJson(groupJson(max: 3)).isMultiSelect, isTrue);
    });

    test('an unorderable item carries the code for why', () {
      final PublicItem item = PublicItem.fromJson(itemJson(orderable: false));
      expect(item.orderable, isFalse);
      expect(item.unavailableReason, 'out_of_stock');
      expect(item.allGroups, isEmpty);
    });

    test('groups list variants before add-ons', () {
      final PublicItem item = PublicItem.fromJson(
        itemJson(variantGroups: <Map<String, Object?>>[groupJson()]),
      );
      expect(item.allGroups.single.name, 'সাইজ');
    });

    test('a menu keeps the shop\'s own order and groups by category', () {
      final Menu menu = Menu.fromJson(menuJson());
      expect(menu.isEmpty, isFalse);
      expect(menu.categories.single.sortOrder, 1);
      expect(menu.itemsIn(menu.categories.single).single.name, 'কাচ্চি');
      expect(Menu.fromJson(menuJson(items: <Map<String, Object?>>[])).isEmpty,
          isTrue);
    });

    test('savings is the server\'s figure, or absent', () {
      expect(Combo.fromJson(comboJson()).savings?.display, '৳ ৪০');
      expect(Combo.fromJson(comboJson(withSavings: false)).savings, isNull);
    });
  });

  group('cart', () {
    test('a line states its own problem in the server\'s words', () {
      final Cart cart = Cart.fromJson(cartJson(lineHasIssue: true));
      final CartLine line = cart.lines.single;
      expect(line.hasIssue, isTrue);
      expect(line.issueText, 'স্টক শেষ');
      expect(line.orderable, isFalse);
      expect(line.options.single.price.display, '৳ ৫০');
    });

    test('an orderable cart has no blocker', () {
      final Cart cart = Cart.fromJson(cartJson());
      expect(cart.orderable, isTrue);
      expect(cart.blocker, isNull);
      expect(cart.isEmpty, isFalse);
      expect(cart.pricing.awayFromFreeDelivery?.display, '৳ ৭০');
    });

    test('a blocked cart says why, in a sentence the server wrote', () {
      final Cart cart = Cart.fromJson(cartJson(orderable: false));
      expect(cart.blocker, 'shop_closed');
      expect(cart.blockerText, 'দোকান বন্ধ');
    });

    test('an empty away-from-free-delivery object reads as null', () {
      final CartPricing pricing = CartPricing.fromJson(<String, Object?>{
        'total': money(0, '৳ ০'),
        'free_delivery': true,
        'rows': <Object?>[],
      });
      expect(pricing.awayFromFreeDelivery, isNull);
      expect(pricing.freeDelivery, isTrue);
    });
  });

  group('order', () {
    test('an order carries the two ids a review needs', () {
      final Order order = Order.fromJson(orderJson(partnerId: 'ptn-1'));
      expect(order.merchantId, 'mer-1');
      expect(order.partnerId, 'ptn-1');
    });

    test('no rider yet means no partner to review', () {
      expect(Order.fromJson(orderJson()).partnerId, isNull);
    });

    test('live and the status decide trackability, not a local list', () {
      expect(Order.fromJson(orderJson()).isTrackable, isTrue);
      expect(
        Order.fromJson(orderJson(status: 'delivered', live: false))
            .isTrackable,
        isFalse,
      );
    });

    test('an unpaid order is not trackable and asks to be paid', () {
      final Order order = Order.fromJson(orderJson(status: 'pending_payment'));
      expect(order.awaitsPayment, isTrue);
      expect(order.isTrackable, isFalse);
    });

    test('the timeline, places and receipt parse whole', () {
      final Order order = Order.fromJson(orderJson());
      expect(order.events.first.label, 'অর্ডার হয়েছে');
      expect(order.events.first.at, isNotNull);
      expect(order.pickup.name, 'Kacchi Bhai');
      expect(order.destination.singleLine, 'রোড ৫, ধানমন্ডি, ঢাকা');
      expect(order.lines.single.lineTotal.display, '৳ ৩২০');
      expect(order.receipt.first.key, 'subtotal');
      expect(order.nextActions, <String>['cancelled']);
      expect(order.placedAt, isNotNull);
    });

    test('next_actions ignores anything that is not a string', () {
      final Map<String, Object?> body = orderJson()
        ..['next_actions'] = <Object?>['cancelled', 7];
      expect(Order.fromJson(body).nextActions, <String>['cancelled']);
      final Map<String, Object?> missing = orderJson()..remove('next_actions');
      expect(Order.fromJson(missing).nextActions, isEmpty);
    });

    test('a closed cancellation window states its reason', () {
      final OrderCancellation cancel = OrderCancellation.fromJson(
        cancellationJson(allowed: false),
      );
      expect(cancel.allowed, isFalse);
      expect(cancel.reason, 'window_closed');
      expect(cancel.secondsLeft, 0);
    });
  });

  group('account', () {
    test('display_name is never empty, even when name is', () {
      expect(Profile.fromJson(profileJson(name: '')).hasName, isFalse);
      expect(
        Profile.fromJson(profileJson(name: '')).displayName,
        isNotEmpty,
      );
    });

    test('an address carries the server\'s one-line rendering', () {
      final Address address = Address.fromJson(addressJson());
      expect(address.singleLine, 'রোড ৫, ধানমন্ডি, ঢাকা');
      expect(address.isDefault, isTrue);
      expect(address.lat, 23.7461);
    });

    test('an area names the division D3 will not let a search cross', () {
      expect(Area.fromJson(areaJson()).divisionCode, 'DHA');
    });
  });

  group('tracking', () {
    test('no partner yet is a frame without one', () {
      final TrackingSnapshot snapshot = TrackingSnapshot.fromJson(
        <String, Object?>{
          'order_id': 'ord-1',
          'status': 'accepted',
          'status_label': 'গ্রহণ করা হয়েছে',
          'live': true,
        },
      );
      expect(snapshot.hasPartner, isFalse);
      expect(snapshot.partner, isNull);
    });

    test('a carrying frame has the rider and a position', () {
      final TrackingSnapshot snapshot = TrackingSnapshot.fromJson(
        <String, Object?>{
          'order_id': 'ord-1',
          'status': 'picked_up',
          'status_label': 'পথে আছে',
          'live': true,
          'partner': <String, Object?>{
            'id': 'ptn-1',
            'name': 'করিম',
            'phone': '01811111111',
            'vehicle': 'bike',
            'lat': 23.75,
            'lng': 90.38,
          },
        },
      );
      expect(snapshot.hasPartner, isTrue);
      expect(snapshot.partner!.name, 'করিম');
      expect(snapshot.partner!.lat, 23.75);
    });
  });

  group('payment', () {
    test('the manual gateway returns no page to send anyone to', () {
      expect(Checkout.fromJson(checkoutJson()).hasRedirect, isFalse);
      expect(
        Checkout.fromJson(checkoutJson(redirect: 'http://pay.test')).redirectUrl,
        'http://pay.test',
      );
    });

    test('a payment states capture and failure separately', () {
      expect(Payment.fromJson(paymentJson()).isCaptured, isTrue);
      final Payment failed = Payment.fromJson(
        paymentJson(status: 'failed', reason: 'কার্ড বাতিল'),
      );
      expect(failed.isFailed, isTrue);
      expect(failed.reason, 'কার্ড বাতিল');
    });
  });

  group('reviews and support', () {
    test('a subject this build has never heard of is not a crash', () {
      expect(ReviewSubject.parse('partner'), ReviewSubject.partner);
      expect(ReviewSubject.parse('item'), ReviewSubject.item);
      expect(ReviewSubject.parse('spaceship'), ReviewSubject.merchant);
    });

    test('a review parses whole', () {
      final Review review = Review.fromJson(reviewJson());
      expect(review.rating, 5);
      expect(review.subject, ReviewSubject.merchant);
      expect(review.createdAt, isNotNull);
    });

    test('a subject nobody has rated says so rather than showing 0.0', () {
      expect(Rating.fromJson(ratingJson()).hasRatings, isTrue);
      expect(Rating.fromJson(ratingJson(count: 0)).hasRatings, isFalse);
      expect(Rating.fromJson(ratingJson()).average, 4.5);
    });

    test('a resolved ticket says whether it was refunded, not how much', () {
      final SupportTicket open = SupportTicket.fromJson(ticketJson());
      expect(open.isOpen, isTrue);
      expect(open.resolution, isNull);
      final SupportTicket resolved = SupportTicket.fromJson(
        ticketJson(status: 'resolved'),
      );
      expect(resolved.isOpen, isFalse);
      expect(resolved.wasRefunded, isTrue);
      expect(resolved.resolvedAt, isNotNull);
    });
  });

  group('notifications', () {
    test('a notification that never arrived is marked', () {
      expect(AppNotification.fromJson(notificationJson()).didFail, isFalse);
      final AppNotification failed = AppNotification.fromJson(
        notificationJson(status: 'failed'),
      );
      expect(failed.didFail, isTrue);
      expect(failed.title, 'অর্ডার গ্রহণ হয়েছে');
      expect(failed.createdAt, isNotNull);
    });
  });

  test('money is carried, never computed', () {
    final Money amount = Money.fromJson(money(32000, '৳ ৩২০'));
    expect(amount.minor, 32000);
    expect(amount.display, '৳ ৩২০');
  });
}
