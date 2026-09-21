import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_merchant/src/api/models/catalogue.dart';
import 'package:goklay_merchant/src/api/models/order.dart';
import 'package:goklay_merchant/src/dependencies.dart';
import 'package:goklay_merchant/src/l10n/merchant_strings.dart';
import 'package:goklay_merchant/src/screens/account_screen.dart';
import 'package:goklay_merchant/src/screens/board_screen.dart';
import 'package:goklay_merchant/src/screens/catalogue_screen.dart';
import 'package:goklay_merchant/src/screens/item_form_screen.dart';
import 'package:goklay_merchant/src/screens/order_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const MerchantStringsBn bn = MerchantStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('the board', () {
    Future<Dependencies> pumpBoard(
      WidgetTester tester, {
      void Function(MerchantOrder)? onSelected,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        BoardScreen(
          merchantId: 'mch-1',
          onOrderSelected: onSelected ?? (_) {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('a new order offers exactly what next_actions names', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      final Dependencies dependencies = await pumpBoard(tester);
      expect(find.text('GK-7F3K'), findsOneWidget);
      expect(find.text(bn.payOnDelivery), findsOneWidget);
      expect(find.text(bn.accept), findsOneWidget);
      expect(find.text(bn.reject), findsOneWidget);
      expect(find.text(bn.startPreparing), findsNothing);
      expect(find.text(bn.markReady), findsNothing);
      dependencies.dispose();
    });

    testWidgets('an accepted order offers preparing, not accept', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders',
        (_) => orderListJson(
          orders: <Map<String, Object?>>[
            orderJson(
              status: 'accepted',
              nextActions: const <String>['preparing'],
            ),
          ],
        ),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      expect(find.text(bn.startPreparing), findsOneWidget);
      expect(find.text(bn.accept), findsNothing);
      dependencies.dispose();
    });

    testWidgets('accepting posts and re-reads the board', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/accept',
        (_) => orderJson(status: 'accepted'),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.accept),
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/accept'),
        hasLength(1),
      );
      expect(backend.to('/v1/merchants/mch-1/orders'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('ready is offered when the state machine allows it', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders',
        (_) => orderListJson(
          orders: <Map<String, Object?>>[
            orderJson(
              status: 'preparing',
              nextActions: const <String>['ready'],
            ),
          ],
        ),
      );
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/ready',
        (_) => orderJson(status: 'ready'),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.markReady),
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/ready'),
        hasLength(1),
      );
      dependencies.dispose();
    });

    testWidgets('preparing is posted from the board too', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders',
        (_) => orderListJson(
          orders: <Map<String, Object?>>[
            orderJson(
              status: 'accepted',
              nextActions: const <String>['preparing'],
            ),
          ],
        ),
      );
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/preparing',
        (_) => orderJson(status: 'preparing'),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.startPreparing),
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/preparing'),
        hasLength(1),
      );
      dependencies.dispose();
    });

    testWidgets('rejecting from the board opens the order, for the reason', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      MerchantOrder? opened;
      final Dependencies dependencies = await pumpBoard(
        tester,
        onSelected: (MerchantOrder order) => opened = order,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.reject),
      );
      expect(opened!.id, 'ord-1');
      expect(backend.to('/v1/merchants/mch-1/orders/ord-1/reject'), isEmpty);
      dependencies.dispose();
    });

    testWidgets('tapping an order opens it', (WidgetTester tester) async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      MerchantOrder? opened;
      final Dependencies dependencies = await pumpBoard(
        tester,
        onSelected: (MerchantOrder order) => opened = order,
      );
      await tester.tap(find.bySemanticsLabel('GK-7F3K'));
      await tester.pumpAndSettle();
      expect(opened, isNotNull);
      dependencies.dispose();
    });

    testWidgets('the past tab drops the live filter', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders',
        (_) => orderListJson(orders: <Map<String, Object?>>[]),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      await tester.tap(find.text(bn.boardPast));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/merchants/mch-1/orders').last.url.queryParameters
            .containsKey('live'),
        isFalse,
      );
      expect(find.text(bn.boardEmpty), findsOneWidget);
      await tester.tap(find.text(bn.boardLive));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/merchants/mch-1/orders').last.url
            .queryParameters['live'],
        'true',
      );
      dependencies.dispose();
    });

    testWidgets('a refused transition is shown', (WidgetTester tester) async {
      route('GET /v1/merchants/mch-1/orders', (_) => orderListJson());
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/accept',
        (_) => errorBody('already_cancelled', 'অর্ডারটি বাতিল হয়ে গেছে'),
      );
      backend.statuses['POST /v1/merchants/mch-1/orders/ord-1/accept'] = 409;
      final Dependencies dependencies = await pumpBoard(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.accept),
      );
      expect(find.text('অর্ডারটি বাতিল হয়ে গেছে'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route('GET /v1/merchants/mch-1/orders', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['GET /v1/merchants/mch-1/orders'] = 503;
      final Dependencies dependencies = await pumpBoard(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/merchants/mch-1/orders'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('an online order is marked as already paid', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders',
        (_) => orderListJson(
          orders: <Map<String, Object?>>[orderJson(payment: 'online')],
        ),
      );
      final Dependencies dependencies = await pumpBoard(tester);
      expect(find.text(bn.paidOnline), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('one order', () {
    Future<Dependencies> pumpOrder(WidgetTester tester) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const OrderScreen(merchantId: 'mch-1', orderId: 'ord-1'),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('shows the lines, the note, the address and the receipt', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders/ord-1', (_) => orderJson());
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text('2 × কাচ্চি'), findsOneWidget);
      expect(find.text('বড়'), findsOneWidget);
      expect(find.text('ঝাল কম'), findsOneWidget);
      await reveal(tester, find.text('রোড ৫, ধানমন্ডি, ঢাকা'));
      expect(find.text('রোড ৫, ধানমন্ডি, ঢাকা'), findsOneWidget);
      await reveal(tester, find.text('সাবটোটাল'));
      expect(find.text('সাবটোটাল'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('rejecting needs a reason, and sends the shop\'s words', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders/ord-1', (_) => orderJson());
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/reject',
        (_) => orderJson(
          status: 'rejected',
          live: false,
          nextActions: const <String>[],
        ),
      );
      final Dependencies dependencies = await pumpOrder(tester);
      await reveal(tester, find.widgetWithText(GoklayButton, bn.reject));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.reject),
            )
            .isEnabled,
        isFalse,
      );
      await enterInto(tester, bn.rejectReason, 'উপকরণ নেই');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.reject),
      );
      expect(
        (backend.to('/v1/merchants/mch-1/orders/ord-1/reject').single.body!
            as Map<String, Object?>)['reason'],
        'উপকরণ নেই',
      );
      dependencies.dispose();
    });

    testWidgets('accepting from the detail screen repaints it', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders/ord-1', (_) => orderJson());
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/accept',
        (_) => orderJson(
          status: 'accepted',
          nextActions: const <String>['preparing'],
        ),
      );
      final Dependencies dependencies = await pumpOrder(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.accept),
      );
      await reveal(tester, find.widgetWithText(GoklayButton, bn.startPreparing));
      expect(
        find.widgetWithText(GoklayButton, bn.startPreparing),
        findsOneWidget,
      );
      dependencies.dispose();
    });

    testWidgets('preparing and ready are both posted from here', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders/ord-1',
        (_) => orderJson(
          status: 'accepted',
          nextActions: const <String>['preparing', 'ready'],
        ),
      );
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/preparing',
        (_) => orderJson(
          status: 'preparing',
          nextActions: const <String>['ready'],
        ),
      );
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/ready',
        (_) => orderJson(
          status: 'ready',
          nextActions: const <String>[],
        ),
      );
      final Dependencies dependencies = await pumpOrder(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.startPreparing),
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.markReady),
      );
      expect(
        backend.to('/v1/merchants/mch-1/orders/ord-1/ready'),
        hasLength(1),
      );
      dependencies.dispose();
    });

    testWidgets('a refusal is shown and the order stands', (
      WidgetTester tester,
    ) async {
      route('GET /v1/merchants/mch-1/orders/ord-1', (_) => orderJson());
      route(
        'POST /v1/merchants/mch-1/orders/ord-1/accept',
        (_) => errorBody('gone', 'অর্ডারটি আর নেই'),
      );
      backend.statuses['POST /v1/merchants/mch-1/orders/ord-1/accept'] = 409;
      final Dependencies dependencies = await pumpOrder(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.accept),
      );
      expect(find.text('অর্ডারটি আর নেই'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an order paid online is marked as such here too', (
      WidgetTester tester,
    ) async {
      route(
        'GET /v1/merchants/mch-1/orders/ord-1',
        (_) => orderJson(payment: 'online'),
      );
      final Dependencies dependencies = await pumpOrder(tester);
      expect(find.text(bn.paidOnline), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed read offers a retry', (WidgetTester tester) async {
      route(
        'GET /v1/merchants/mch-1/orders/ord-1',
        (_) => errorBody('not_found', 'পাওয়া যায়নি'),
      );
      backend.statuses['GET /v1/merchants/mch-1/orders/ord-1'] = 404;
      final Dependencies dependencies = await pumpOrder(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/merchants/mch-1/orders/ord-1'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('the catalogue', () {
    Future<Dependencies> pumpCatalogue(
      WidgetTester tester, {
      Future<bool> Function(CatalogueCapabilities)? onAddItem,
      String type = 'restaurant',
    }) async {
      route(
        'GET /v1/merchants/mch-1/catalogue/capabilities',
        (_) => capabilitiesJson(type: type),
      );
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        CatalogueScreen(
          merchantId: 'mch-1',
          onAddItem: onAddItem ?? (_) async => false,
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    void catalogueRoutes({
      List<Map<String, Object?>>? items,
      List<Map<String, Object?>>? categories,
      List<Map<String, Object?>>? combos,
    }) {
      route(
        'GET /v1/merchants/mch-1/catalogue/categories',
        (_) => <String, Object?>{
          'categories': categories ?? <Map<String, Object?>>[categoryJson()],
        },
      );
      route(
        'GET /v1/merchants/mch-1/catalogue/items',
        (_) => <String, Object?>{
          'items': items ?? <Map<String, Object?>>[itemJson()],
        },
      );
      route(
        'GET /v1/merchants/mch-1/catalogue/combos',
        (_) => <String, Object?>{
          'combos': combos ?? <Map<String, Object?>>[comboJson()],
        },
      );
    }

    testWidgets('a restaurant gets a bundles tab and no shelf', (
      WidgetTester tester,
    ) async {
      catalogueRoutes();
      final Dependencies dependencies = await pumpCatalogue(tester);
      expect(find.text(bn.combos), findsOneWidget);
      expect(find.text('কাচ্চি'), findsOneWidget);
      expect(find.text(bn.stockQuantity), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a grocery gets a shelf and no bundles tab', (
      WidgetTester tester,
    ) async {
      catalogueRoutes(
        items: <Map<String, Object?>>[
          itemJson(stockTracked: true, stock: 4),
        ],
      );
      route(
        'PUT /v1/merchants/mch-1/catalogue/items/itm-1/stock',
        (_) => itemJson(stockTracked: true, stock: 5),
      );
      final Dependencies dependencies = await pumpCatalogue(
        tester,
        type: 'grocery',
      );
      expect(find.text(bn.combos), findsNothing);
      expect(find.text(bn.stockQuantity), findsOneWidget);
      expect(backend.to('/v1/merchants/mch-1/catalogue/combos'), isEmpty);
      await tester.tap(find.bySemanticsLabel(core.increaseQuantity));
      await tester.pumpAndSettle();
      expect(
        (backend.to('/v1/merchants/mch-1/catalogue/items/itm-1/stock').single
            .body! as Map<String, Object?>)['quantity'],
        5,
      );
      dependencies.dispose();
    });

    testWidgets('an item is hidden and a hidden one is marked', (
      WidgetTester tester,
    ) async {
      catalogueRoutes();
      route(
        'PUT /v1/merchants/mch-1/catalogue/items/itm-1/active',
        (_) => itemJson(active: false, orderable: false),
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      await revealAndTap(tester, find.text(bn.hide).first);
      expect(
        (backend.to('/v1/merchants/mch-1/catalogue/items/itm-1/active').single
            .body! as Map<String, Object?>)['active'],
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('an item that is on but unorderable says so', (
      WidgetTester tester,
    ) async {
      catalogueRoutes(
        items: <Map<String, Object?>>[itemJson(orderable: false)],
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      expect(find.text(bn.notOrderable), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a hidden item is marked hidden', (WidgetTester tester) async {
      catalogueRoutes(
        items: <Map<String, Object?>>[
          itemJson(active: false, orderable: false),
        ],
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      expect(find.text(bn.hidden), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an empty catalogue says so', (WidgetTester tester) async {
      catalogueRoutes(items: <Map<String, Object?>>[]);
      final Dependencies dependencies = await pumpCatalogue(tester);
      expect(find.text(bn.catalogueEmpty), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a section is added and another hidden', (
      WidgetTester tester,
    ) async {
      catalogueRoutes();
      route(
        'POST /v1/merchants/mch-1/catalogue/categories',
        (_) => categoryJson(id: 'cat-2', name: 'পানীয়'),
      );
      route(
        'PUT /v1/merchants/mch-1/catalogue/categories/cat-1/active',
        (_) => categoryJson(active: false),
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      await tester.tap(find.text(bn.sections));
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.text(bn.hide));
      expect(
        backend.to('/v1/merchants/mch-1/catalogue/categories/cat-1/active'),
        hasLength(1),
      );
      await enterInto(tester, bn.addSection, 'পানীয়');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addSection),
      );
      final SentRequest posted = backend
          .to('/v1/merchants/mch-1/catalogue/categories')
          .lastWhere((SentRequest request) => request.method == 'POST');
      expect((posted.body! as Map<String, Object?>)['name'], 'পানীয়');
      dependencies.dispose();
    });

    testWidgets('a hidden section and a hidden bundle offer "show"', (
      WidgetTester tester,
    ) async {
      catalogueRoutes(
        categories: <Map<String, Object?>>[categoryJson(active: false)],
        combos: <Map<String, Object?>>[
          comboJson(active: false, orderable: false),
        ],
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      await tester.tap(find.text(bn.sections));
      await tester.pumpAndSettle();
      expect(find.text(bn.show), findsOneWidget);
      await tester.tap(find.text(bn.combos));
      await tester.pumpAndSettle();
      expect(find.text(bn.notOrderable), findsOneWidget);
      expect(find.text(bn.show), findsOneWidget);
      // Back to the items tab, which is the one the screen opens on and so
      // the one nothing has had to tap before.
      await tester.tap(find.text(bn.items));
      await tester.pumpAndSettle();
      expect(find.text('কাচ্চি'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a bundle is switched off from its own tab', (
      WidgetTester tester,
    ) async {
      catalogueRoutes();
      route(
        'PUT /v1/merchants/mch-1/catalogue/combos/cmb-1/active',
        (_) => comboJson(active: false),
      );
      final Dependencies dependencies = await pumpCatalogue(tester);
      await tester.tap(find.text(bn.combos));
      await tester.pumpAndSettle();
      expect(find.text('পরিবার প্যাক'), findsOneWidget);
      expect(find.text('কাচ্চি'), findsOneWidget);
      await revealAndTap(tester, find.text(bn.hide));
      expect(
        backend.to('/v1/merchants/mch-1/catalogue/combos/cmb-1/active'),
        hasLength(1),
      );
      dependencies.dispose();
    });

    testWidgets('an unorderable bundle says so, and an empty list too', (
      WidgetTester tester,
    ) async {
      catalogueRoutes(combos: <Map<String, Object?>>[]);
      final Dependencies dependencies = await pumpCatalogue(tester);
      await tester.tap(find.text(bn.combos));
      await tester.pumpAndSettle();
      expect(find.text(bn.catalogueEmpty), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('adding an item re-reads, declining does not', (
      WidgetTester tester,
    ) async {
      catalogueRoutes();
      bool added = false;
      final Dependencies dependencies = await pumpCatalogue(
        tester,
        onAddItem: (_) async => added,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(backend.to('/v1/merchants/mch-1/catalogue/items'), hasLength(1));
      added = true;
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(backend.to('/v1/merchants/mch-1/catalogue/items'), hasLength(2));
      dependencies.dispose();
    });

    testWidgets('a refused change is shown', (WidgetTester tester) async {
      catalogueRoutes();
      route(
        'PUT /v1/merchants/mch-1/catalogue/items/itm-1/active',
        (_) => errorBody('not_yours', 'এই দোকান আপনার নয়'),
      );
      backend.statuses['PUT /v1/merchants/mch-1/catalogue/items/itm-1/active'] =
          403;
      final Dependencies dependencies = await pumpCatalogue(tester);
      await revealAndTap(tester, find.text(bn.hide).first);
      expect(find.text('এই দোকান আপনার নয়'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a failed capabilities read offers a retry', (
      WidgetTester tester,
    ) async {
      backend.routes['GET /v1/merchants/mch-1/catalogue/capabilities'] =
          (_) => errorBody('b', 'সমস্যা');
      backend.statuses['GET /v1/merchants/mch-1/catalogue/capabilities'] = 503;
      catalogueRoutes();
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        CatalogueScreen(merchantId: 'mch-1', onAddItem: (_) async => false),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(
        backend.to('/v1/merchants/mch-1/catalogue/capabilities'),
        hasLength(2),
      );
      dependencies.dispose();
    });
  });

  group('the item form', () {
    Future<Dependencies> pumpForm(
      WidgetTester tester, {
      String type = 'restaurant',
      List<Map<String, Object?>>? categories,
      void Function(OwnerItem)? onAdded,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        ItemFormScreen(
          merchantId: 'mch-1',
          capabilities: CatalogueCapabilities.fromJson(
            capabilitiesJson(type: type),
          ),
          categories: <OwnerCategory>[
            for (final Map<String, Object?> json
                in categories ?? <Map<String, Object?>>[categoryJson()])
              OwnerCategory.fromJson(json),
          ],
          onAdded: onAdded ?? (_) {},
        ),
        dependencies: dependencies,
      );
      return dependencies;
    }

    testWidgets('a restaurant form has no unit and no prescription', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpForm(tester);
      expect(find.text(bn.unitOfSale), findsNothing);
      expect(find.text(bn.requiresPrescription), findsNothing);
      dependencies.dispose();
    });

    testWidgets('a pharmacy form has both, and sends them', (
      WidgetTester tester,
    ) async {
      route('POST /v1/merchants/mch-1/catalogue/items', (_) => itemJson());
      final Dependencies dependencies = await pumpForm(
        tester,
        type: 'pharmacy',
      );
      expect(find.text(bn.unitOfSale), findsOneWidget);
      await revealAndTap(tester, find.text(bn.requiresPrescription));
      await revealAndTap(tester, find.text('bottle'));
      await enterInto(tester, bn.itemName, 'নাপা');
      await enterInto(tester, bn.itemPriceMinor, '1200');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      final Map<String, Object?> body = backend
          .to('/v1/merchants/mch-1/catalogue/items')
          .single
          .body! as Map<String, Object?>;
      expect(body['price_minor'], 1200);
      expect(body['unit'], 'bottle');
      expect(body['requires_prescription'], isTrue);
      dependencies.dispose();
    });

    testWidgets('nothing is sent without a name and a whole price', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpForm(tester);
      await reveal(tester, find.widgetWithText(GoklayButton, bn.addItem));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.addItem),
            )
            .isEnabled,
        isFalse,
      );
      await enterInto(tester, bn.itemName, 'কাচ্চি');
      await enterInto(tester, bn.itemPriceMinor, '৩২০');
      await reveal(tester, find.widgetWithText(GoklayButton, bn.addItem));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.addItem),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('with no sections there is nothing to file it under', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpForm(
        tester,
        categories: <Map<String, Object?>>[],
      );
      await enterInto(tester, bn.itemName, 'কাচ্চি');
      await enterInto(tester, bn.itemPriceMinor, '32000');
      await reveal(tester, find.widgetWithText(GoklayButton, bn.addItem));
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.addItem),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('a second section can be chosen', (WidgetTester tester) async {
      route('POST /v1/merchants/mch-1/catalogue/items', (_) => itemJson());
      final Dependencies dependencies = await pumpForm(
        tester,
        categories: <Map<String, Object?>>[
          categoryJson(),
          categoryJson(id: 'cat-2', name: 'পানীয়'),
        ],
      );
      await revealAndTap(tester, find.text('পানীয়'));
      await enterInto(tester, bn.itemName, 'চা');
      await enterInto(tester, bn.itemPriceMinor, '2000');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(
        (backend.to('/v1/merchants/mch-1/catalogue/items').single.body!
            as Map<String, Object?>)['category_id'],
        'cat-2',
      );
      dependencies.dispose();
    });

    testWidgets('a refusal is shown', (WidgetTester tester) async {
      route(
        'POST /v1/merchants/mch-1/catalogue/items',
        (_) => errorBody('field_not_allowed', 'এই ঘরটি এই দোকানে চলে না'),
      );
      backend.statuses['POST /v1/merchants/mch-1/catalogue/items'] = 400;
      final Dependencies dependencies = await pumpForm(tester);
      await enterInto(tester, bn.itemName, 'কাচ্চি');
      await enterInto(tester, bn.itemPriceMinor, '32000');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addItem),
      );
      expect(find.text('এই ঘরটি এই দোকানে চলে না'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('the account', () {
    testWidgets('signing out clears the tokens even when the call fails', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      route('POST /v1/auth/logout', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['POST /v1/auth/logout'] = 503;
      final Dependencies dependencies = await harnessDependencies(backend);
      int signedOut = 0;
      await pumpScreen(
        tester,
        MerchantAccountScreen(
          onLanguage: () {},
          onSignedOut: () => signedOut += 1,
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text('করিম'), findsOneWidget);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.signOut),
      );
      expect(signedOut, 1);
      expect(dependencies.session.isSignedIn, isFalse);
      dependencies.dispose();
    });

    testWidgets('the language row opens the picker', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      int opened = 0;
      await pumpScreen(
        tester,
        MerchantAccountScreen(
          onLanguage: () => opened += 1,
          onSignedOut: () {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, 'ভাষা / Language'),
      );
      expect(opened, 1);
      dependencies.dispose();
    });

    testWidgets('switching language changes the requests too', (
      WidgetTester tester,
    ) async {
      route('PATCH /v1/me', (_) => profileJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      Locale? chosen;
      await pumpScreen(
        tester,
        MerchantLanguageScreen(
          onChanged: (Locale locale) => chosen = locale,
        ),
        dependencies: dependencies,
      );
      await tester.tap(find.widgetWithText(SettingRow, 'English'));
      await tester.pumpAndSettle();
      expect(chosen, const Locale('en'));
      expect(dependencies.locale, const Locale('en'));
      expect(backend.to('/v1/me').last.url.queryParameters['lang'], 'en');
      await tester.tap(find.widgetWithText(SettingRow, 'বাংলা'));
      await tester.pumpAndSettle();
      expect(dependencies.locale, const Locale('bn'));
      dependencies.dispose();
    });

    testWidgets('a failed profile read offers a retry', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => errorBody('b', 'সমস্যা'));
      backend.statuses['GET /v1/me'] = 503;
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        MerchantAccountScreen(onLanguage: () {}, onSignedOut: () {}),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/me'), hasLength(2));
      dependencies.dispose();
    });
  });
}
