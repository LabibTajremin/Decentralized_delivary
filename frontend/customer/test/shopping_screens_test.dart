import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/api/models/cart.dart';
import 'package:goklay_customer/src/api/models/catalogue.dart';
import 'package:goklay_customer/src/api/models/discovery.dart';
import 'package:goklay_customer/src/api/models/order.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/cart_screen.dart';
import 'package:goklay_customer/src/screens/home_screen.dart';
import 'package:goklay_customer/src/screens/item_screen.dart';
import 'package:goklay_customer/src/screens/place_order_screen.dart';
import 'package:goklay_customer/src/screens/review_cart_screen.dart';
import 'package:goklay_customer/src/screens/shop_screen.dart';
import 'package:goklay_customer/src/widgets/cards.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('home', () {
    Future<Dependencies> pumpHome(
      WidgetTester tester, {
      void Function(DiscoveryMerchant)? onShop,
      VoidCallback? onAddAddress,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        HomeScreen(
          onShopSelected: onShop ?? (_) {},
          onAddAddress: onAddAddress ?? () {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('searches around the default address, not the first one', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(id: 'adr-1', isDefault: false),
          <String, Object?>{
            ...addressJson(id: 'adr-2'),
            'lat': 24.0,
            'lng': 91.0,
          },
        ],
      });
      route('GET /v1/discovery/merchants', (_) => searchJson());
      final Dependencies dependencies = await pumpHome(tester);
      final Uri url = backend.to('/v1/discovery/merchants').single.url;
      expect(url.queryParameters['lat'], '24.0');
      expect(find.text('Kacchi Bhai'), findsOneWidget);
      expect(find.text('কাছের দোকান'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('with no default it falls back to the first address', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[addressJson(isDefault: false)],
      });
      route('GET /v1/discovery/merchants', (_) => searchJson());
      final Dependencies dependencies = await pumpHome(tester);
      expect(backend.to('/v1/discovery/merchants'), hasLength(1));
      dependencies.dispose();
    });

    testWidgets('with no address at all it asks for one', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[]});
      int asked = 0;
      final Dependencies dependencies = await pumpHome(
        tester,
        onAddAddress: () => asked += 1,
      );
      expect(find.text(bn.noAddresses), findsOneWidget);
      expect(backend.to('/v1/discovery/merchants'), isEmpty);
      await tester.tap(find.bySemanticsLabel(bn.addAddress));
      await tester.pumpAndSettle();
      expect(asked, 1);
      dependencies.dispose();
    });

    testWidgets('a failed address read offers a retry', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => errorBody('boom', 'সমস্যা হয়েছে'));
      backend.statuses['GET /v1/me/addresses'] = 503;
      final Dependencies dependencies = await pumpHome(tester);
      expect(find.text('সমস্যা হয়েছে'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/me/addresses'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('a type chip re-searches from level 0', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('GET /v1/discovery/merchants', (_) => searchJson());
      final Dependencies dependencies = await pumpHome(tester);
      await tester.tap(find.bySemanticsLabel(bn.typePharmacy));
      await tester.pumpAndSettle();
      final Uri url = backend.to('/v1/discovery/merchants').last.url;
      expect(url.queryParameters['type'], 'pharmacy');
      expect(url.queryParameters['level'], '0');

      await tester.tap(find.bySemanticsLabel(bn.typeAll));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/discovery/merchants').last.url.queryParameters
            .containsKey('type'),
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('searching by name submits the query', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('GET /v1/discovery/merchants', (_) => searchJson());
      final Dependencies dependencies = await pumpHome(tester);
      await tester.enterText(find.byType(TextField), 'kacchi');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/discovery/merchants').last.url.queryParameters['q'],
        'kacchi',
      );
      dependencies.dispose();
    });

    testWidgets('widening sends the level the server named', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('GET /v1/discovery/merchants', (_) => searchJson());
      final Dependencies dependencies = await pumpHome(tester);
      await tester.tap(find.bySemanticsLabel(bn.searchWider));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/discovery/merchants').last.url.queryParameters['level'],
        '1',
      );
      dependencies.dispose();
    });

    testWidgets('at the division ceiling widening is not offered (D3)', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route(
        'GET /v1/discovery/merchants',
        (_) => searchJson(atCeiling: true, merchants: <Map<String, Object?>>[]),
      );
      final Dependencies dependencies = await pumpHome(tester);
      expect(find.text(bn.searchWider), findsNothing);
      expect(find.text(bn.noShops), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a shop card opens the shop', (WidgetTester tester) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('GET /v1/discovery/merchants', (_) => searchJson());
      DiscoveryMerchant? opened;
      final Dependencies dependencies = await pumpHome(
        tester,
        onShop: (DiscoveryMerchant merchant) => opened = merchant,
      );
      await tester.tap(find.byType(ShopCard));
      await tester.pumpAndSettle();
      expect(opened!.id, 'mer-1');
      dependencies.dispose();
    });
  });

  group('shop', () {
    Future<Dependencies> pumpShop(
      WidgetTester tester, {
      void Function(PublicItem)? onItem,
      VoidCallback? onReviews,
      bool open = true,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ShopScreen(
          merchant: DiscoveryMerchant.fromJson(merchantJson(open: open)),
          onItemSelected: onItem ?? (_) {},
          onReviews: onReviews ?? () {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('lists categories, items and bundles', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/catalogue/mer-1/menu',
        (_) => menuJson(combos: <Map<String, Object?>>[comboJson()]),
      );
      PublicItem? opened;
      final Dependencies dependencies = await pumpShop(
        tester,
        onItem: (PublicItem item) => opened = item,
      );
      expect(find.text('বিরিয়ানি'), findsOneWidget);
      expect(find.text('পরিবার প্যাক'), findsOneWidget);
      expect(find.text('৳ ৪০'), findsOneWidget);
      await tester.tap(find.byType(ItemTile));
      await tester.pumpAndSettle();
      expect(opened!.id, 'itm-1');
      dependencies.dispose();
    });

    testWidgets('a closed shop is said to be closed', (
      WidgetTester tester,
    ) async {
      route('GET /v1/catalogue/mer-1/menu', (_) => menuJson());
      final Dependencies dependencies = await pumpShop(tester, open: false);
      expect(find.text('বন্ধ'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a shop with nothing to sell says so', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/catalogue/mer-1/menu',
        (_) => menuJson(items: <Map<String, Object?>>[]),
      );
      final Dependencies dependencies = await pumpShop(tester);
      expect(find.byType(EmptyView), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('the reviews action opens the reviews', (
      WidgetTester tester,
    ) async {
      route('GET /v1/catalogue/mer-1/menu', (_) => menuJson());
      int opened = 0;
      final Dependencies dependencies = await pumpShop(
        tester,
        onReviews: () => opened += 1,
      );
      await tester.tap(find.bySemanticsLabel(bn.reviews));
      await tester.pumpAndSettle();
      expect(opened, 1);
      dependencies.dispose();
    });
  });

  group('item', () {
    Future<Dependencies> pumpItem(
      WidgetTester tester,
      Map<String, Object?> item, {
      VoidCallback? onAdded,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ItemScreen(
          item: PublicItem.fromJson(item),
          onAdded: onAdded ?? () {},
        ),
        dependencies: dependencies,
      );
      return dependencies;
    }

    testWidgets('a required group must be answered before adding', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items', (_) => cartJson());
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(variantGroups: <Map<String, Object?>>[groupJson()]),
      );
      final Finder add = find.widgetWithText(GoklayButton, bn.addToCart);
      expect(tester.widget<GoklayButton>(add).isEnabled, isFalse);
      await tester.tap(find.text('বড়'));
      await tester.pumpAndSettle();
      expect(tester.widget<GoklayButton>(add).isEnabled, isTrue);
      dependencies.dispose();
    });

    testWidgets('a single-choice group replaces the previous choice', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items', (_) => cartJson());
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(variantGroups: <Map<String, Object?>>[groupJson()]),
      );
      await tester.tap(find.text('ছোট'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('বড়'));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.addToCart));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/cart/items').single.body! as Map<String, Object?>;
      expect(body['choices'], <Object?>[
        <String, Object?>{'group_id': 'grp-1', 'option_id': 'grp-1-b'},
      ]);
      dependencies.dispose();
    });

    testWidgets('tapping a chosen option in a multi group unchooses it', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items', (_) => cartJson());
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(
          variantGroups: <Map<String, Object?>>[
            groupJson(required: false, max: 2),
          ],
        ),
      );
      await tester.tap(find.text('ছোট'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('বড়'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('ছোট'));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.addToCart));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/cart/items').single.body! as Map<String, Object?>;
      expect(body['choices'], hasLength(1));
      dependencies.dispose();
    });

    testWidgets('a full multi group refuses one more', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items', (_) => cartJson());
      // Three add-ons, at most two: the third tap has to be refused rather
      // than silently replacing one, because the shop said two.
      final Map<String, Object?> group = <String, Object?>{
        'id': 'grp-2',
        'name': 'এক্সট্রা',
        'required': false,
        'min_choices': 0,
        'max_choices': 2,
        'options': <Map<String, Object?>>[
          <String, Object?>{
            'id': 'opt-a',
            'name': 'সালাদ',
            'price': money(1000, '৳ ১০'),
            'available': true,
          },
          <String, Object?>{
            'id': 'opt-b',
            'name': 'বোরহানি',
            'price': money(3000, '৳ ৩০'),
            'available': true,
          },
          <String, Object?>{
            'id': 'opt-c',
            'name': 'ডিম',
            'price': money(2000, '৳ ২০'),
            'available': true,
          },
        ],
      };
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(variantGroups: <Map<String, Object?>>[group]),
      );
      await tester.tap(find.text('সালাদ'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('বোরহানি'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('ডিম'));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.addToCart));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/cart/items').single.body! as Map<String, Object?>;
      expect(body['choices'], <Object?>[
        <String, Object?>{'group_id': 'grp-2', 'option_id': 'opt-a'},
        <String, Object?>{'group_id': 'grp-2', 'option_id': 'opt-b'},
      ]);
      dependencies.dispose();
    });

    testWidgets('an unavailable option cannot be chosen', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(
          variantGroups: <Map<String, Object?>>[groupJson(available: false)],
        ),
      );
      await tester.tap(find.text('ছোট'));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.addToCart),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('an unorderable item cannot be added at all', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(orderable: false),
      );
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.addToCart),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('quantity and note go with the line', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items', (_) => cartJson());
      int added = 0;
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(),
        onAdded: () => added += 1,
      );
      await tester.tap(find.bySemanticsLabel(core.increaseQuantity));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'ঝাল কম');
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.addToCart));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/cart/items').single.body! as Map<String, Object?>;
      expect(body['quantity'], 2);
      expect(body['note'], 'ঝাল কম');
      expect(added, 1);
      dependencies.dispose();
    });

    testWidgets('a refused line shows the server\'s reason and stays put', (
      WidgetTester tester,
    ) async {
      route('POST /v1/cart/items',
          (_) => errorBody('other_merchant', 'অন্য দোকানের কার্ট আছে'));
      backend.statuses['POST /v1/cart/items'] = 409;
      int added = 0;
      final Dependencies dependencies = await pumpItem(
        tester,
        itemJson(),
        onAdded: () => added += 1,
      );
      await tester.tap(find.bySemanticsLabel(bn.addToCart));
      await tester.pumpAndSettle();
      expect(find.text('অন্য দোকানের কার্ট আছে'), findsOneWidget);
      expect(added, 0);
      dependencies.dispose();
    });

    testWidgets('pharmacy and grocery fields print as they arrived', (
      WidgetTester tester,
    ) async {
      final Map<String, Object?> medicine = itemJson()
        ..['brand'] = 'Square'
        ..['generic_name'] = 'Paracetamol'
        ..['strength'] = '500mg'
        ..['pack_size'] = '10 pcs'
        ..['unit'] = 'strip';
      final Dependencies dependencies = await pumpItem(tester, medicine);
      expect(find.text('Square'), findsOneWidget);
      expect(find.text('Paracetamol'), findsOneWidget);
      expect(find.text('500mg'), findsOneWidget);
      expect(find.text('strip'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('cart', () {
    Future<Dependencies> pumpCart(
      WidgetTester tester, {
      void Function(Cart)? onCheckout,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        CartScreen(onCheckout: onCheckout ?? (_) {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('shows the lines, the notice and the receipt', (
      WidgetTester tester,
    ) async {
      route('GET /v1/cart', (_) => cartJson());
      final Dependencies dependencies = await pumpCart(tester);
      expect(find.text('Kacchi Bhai'), findsOneWidget);
      expect(find.text('আর ৳ ৭০ যোগ করলে ডেলিভারি ফ্রি'), findsOneWidget);
      expect(find.text('সাবটোটাল'), findsOneWidget);
      expect(find.text('৳ ৪৩০'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an empty cart says so and offers no checkout', (
      WidgetTester tester,
    ) async {
      route('GET /v1/cart', (_) => cartJson(empty: true));
      final Dependencies dependencies = await pumpCart(tester);
      expect(find.text(bn.cartEmpty), findsOneWidget);
      expect(find.text(bn.reviewCart), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a quantity change round-trips and repaints the server\'s cart',
        (WidgetTester tester) async {
      route('GET /v1/cart', (_) => cartJson());
      route('PUT /v1/cart/lines/lin-1', (_) => cartJson(quantity: 2));
      final Dependencies dependencies = await pumpCart(tester);
      await tester.tap(find.bySemanticsLabel(core.increaseQuantity));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/cart/lines/lin-1'), hasLength(1));
      expect(find.text('2'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('removing a line empties the cart', (
      WidgetTester tester,
    ) async {
      route('GET /v1/cart', (_) => cartJson());
      route('DELETE /v1/cart/lines/lin-1', (_) => cartJson(empty: true));
      final Dependencies dependencies = await pumpCart(tester);
      await tester.tap(find.bySemanticsLabel(bn.removeLine));
      await tester.pumpAndSettle();
      expect(find.text(bn.cartEmpty), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a rejected change is shown, and the cart is unchanged', (
      WidgetTester tester,
    ) async {
      route('GET /v1/cart', (_) => cartJson());
      route('PUT /v1/cart/lines/lin-1',
          (_) => errorBody('out_of_stock', 'স্টক শেষ'));
      backend.statuses['PUT /v1/cart/lines/lin-1'] = 409;
      final Dependencies dependencies = await pumpCart(tester);
      await tester.tap(find.bySemanticsLabel(core.increaseQuantity));
      await tester.pumpAndSettle();
      expect(find.text('স্টক শেষ'), findsOneWidget);
      expect(find.text('1'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a blocked cart shows the blocker but still lets you look', (
      WidgetTester tester,
    ) async {
      route('GET /v1/cart', (_) => cartJson(orderable: false));
      final Dependencies dependencies = await pumpCart(tester);
      expect(find.text('দোকান বন্ধ'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('checkout hands the cart on', (WidgetTester tester) async {
      route('GET /v1/cart', (_) => cartJson());
      Cart? handed;
      final Dependencies dependencies = await pumpCart(
        tester,
        onCheckout: (Cart cart) => handed = cart,
      );
      await tester.tap(find.bySemanticsLabel(bn.reviewCart));
      await tester.pumpAndSettle();
      expect(handed!.id, 'crt-1');
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/cart', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/cart'] = 503;
      final Dependencies dependencies = await pumpCart(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/cart'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('review cart', () {
    Future<Dependencies> pumpReview(
      WidgetTester tester, {
      void Function(Cart, Address)? onContinue,
      Map<String, Object?>? cart,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ReviewCartScreen(
          cart: Cart.fromJson(cart ?? cartJson()),
          onContinue: onContinue ?? (_, _) {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('binds the cart to the default address on open', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('PUT /v1/cart/address', (_) => cartJson(addressId: 'adr-1'));
      final Dependencies dependencies = await pumpReview(tester);
      final Map<String, Object?> body =
          backend.to('/v1/cart/address').single.body! as Map<String, Object?>;
      expect(body['address_id'], 'adr-1');
      expect(body['lat'], 23.7461);
      dependencies.dispose();
    });

    testWidgets('it prefers the address the cart is already bound to', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(),
          addressJson(id: 'adr-2', label: 'অফিস', isDefault: false),
        ],
      });
      route('PUT /v1/cart/address', (_) => cartJson(addressId: 'adr-2'));
      final Dependencies dependencies = await pumpReview(
        tester,
        cart: cartJson(addressId: 'adr-2'),
      );
      final Map<String, Object?> body =
          backend.to('/v1/cart/address').single.body! as Map<String, Object?>;
      expect(body['address_id'], 'adr-2');
      dependencies.dispose();
    });

    testWidgets('choosing another address re-binds and repaints', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(),
          addressJson(id: 'adr-2', label: 'অফিস', isDefault: false),
        ],
      });
      route('PUT /v1/cart/address', (SentRequest request) {
        final Map<String, Object?> body = request.body! as Map<String, Object?>;
        return cartJson(addressId: body['address_id']! as String);
      });
      Address? chosen;
      final Dependencies dependencies = await pumpReview(
        tester,
        onContinue: (Cart _, Address address) => chosen = address,
      );
      await tester.tap(find.text('অফিস'));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/cart/address'), hasLength(2));
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      expect(chosen!.id, 'adr-2');
      dependencies.dispose();
    });

    testWidgets('with no address the step cannot be completed', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[]});
      final Dependencies dependencies = await pumpReview(tester);
      expect(find.text(bn.noAddresses), findsOneWidget);
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.placeOrder),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('a cart the server will not take cannot be placed', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('PUT /v1/cart/address', (_) => cartJson(orderable: false));
      final Dependencies dependencies = await pumpReview(tester);
      expect(find.text('দোকান বন্ধ'), findsOneWidget);
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.placeOrder),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('a failed binding is reported', (WidgetTester tester) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('PUT /v1/cart/address',
          (_) => errorBody('out_of_range', 'এই ঠিকানায় ডেলিভারি হয় না'));
      backend.statuses['PUT /v1/cart/address'] = 400;
      final Dependencies dependencies = await pumpReview(tester);
      expect(find.text('এই ঠিকানায় ডেলিভারি হয় না'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('place order', () {
    Future<Dependencies> pumpPlace(
      WidgetTester tester, {
      void Function(Order)? onPlaced,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        PlaceOrderScreen(
          cart: Cart.fromJson(cartJson(addressId: 'adr-1')),
          address: Address.fromJson(addressJson()),
          onPlaced: onPlaced ?? (_) {},
        ),
        dependencies: dependencies,
      );
      return dependencies;
    }

    testWidgets('defaults to cash and places with an idempotency key', (
      WidgetTester tester,
    ) async {
      route('POST /v1/orders', (_) => orderJson());
      Order? placed;
      final Dependencies dependencies = await pumpPlace(
        tester,
        onPlaced: (Order order) => placed = order,
      );
      expect(find.text('রোড ৫, ধানমন্ডি, ঢাকা'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/orders').single.body! as Map<String, Object?>;
      expect(body['payment_method'], 'cash');
      expect(body['address_id'], 'adr-1');
      expect((body['idempotency_key']! as String), startsWith('crt-1-'));
      expect(placed!.id, 'ord-1');
      dependencies.dispose();
    });

    testWidgets('online is the other of the two methods P13 has', (
      WidgetTester tester,
    ) async {
      route('POST /v1/orders', (_) => orderJson(payment: 'online'));
      final Dependencies dependencies = await pumpPlace(tester);
      await tester.tap(find.text(bn.payOnline));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      final Map<String, Object?> body =
          backend.to('/v1/orders').single.body! as Map<String, Object?>;
      expect(body['payment_method'], 'online');
      dependencies.dispose();
    });

    testWidgets('a refusal is shown and nothing is placed', (
      WidgetTester tester,
    ) async {
      route('POST /v1/orders', (_) => errorBody('cart_empty', 'কার্ট খালি'));
      backend.statuses['POST /v1/orders'] = 409;
      int placed = 0;
      final Dependencies dependencies = await pumpPlace(
        tester,
        onPlaced: (_) => placed += 1,
      );
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      expect(find.text('কার্ট খালি'), findsOneWidget);
      expect(placed, 0);
      dependencies.dispose();
    });

    testWidgets('a retried attempt reuses the same idempotency key', (
      WidgetTester tester,
    ) async {
      route('POST /v1/orders', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['POST /v1/orders'] = 503;
      final Dependencies dependencies = await pumpPlace(tester);
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(bn.placeOrder));
      await tester.pumpAndSettle();
      final List<SentRequest> attempts = backend.to('/v1/orders');
      expect(attempts, hasLength(2));
      expect(
        (attempts.first.body! as Map<String, Object?>)['idempotency_key'],
        (attempts.last.body! as Map<String, Object?>)['idempotency_key'],
      );
      dependencies.dispose();
    });
  });
}
