
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:goklay_customer/src/dependencies.dart';
import 'package:goklay_customer/src/l10n/customer_strings.dart';
import 'package:goklay_customer/src/screens/account_screen.dart';
import 'package:goklay_customer/src/screens/add_address_screen.dart';
import 'package:goklay_customer/src/screens/addresses_screen.dart';
import 'package:goklay_customer/src/screens/language_screen.dart';
import 'package:goklay_customer/src/screens/notifications_screen.dart';
import 'package:goklay_customer/src/screens/placeholder_screen.dart';
import 'package:goklay_customer/src/screens/profile_screen.dart';
import 'package:goklay_customer/src/screens/security_screen.dart';

import 'support/fixtures.dart';
import 'support/harness.dart';

void main() {
  const CustomerStringsBn bn = CustomerStringsBn();
  const GoklayStringsBn core = GoklayStringsBn();
  late FakeBackend backend;

  setUp(() => backend = FakeBackend(<String, Object? Function(SentRequest)>{}));

  void route(String key, Object? Function(SentRequest) handler) =>
      backend.routes[key] = handler;

  group('account', () {
    Future<Dependencies> pumpAccount(
      WidgetTester tester, {
      VoidCallback? onSignedOut,
      void Function(PlaceholderScreen)? onPlaceholder,
      VoidCallback? onProfile,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        AccountScreen(
          onProfile: onProfile ?? () {},
          onAddresses: () {},
          onSecurity: () {},
          onLanguage: () {},
          onNotifications: () {},
          onSupport: () {},
          onPlaceholder: onPlaceholder ?? (_) {},
          onSignedOut: onSignedOut ?? () {},
        ),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('greets with the name the server decided', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson(name: ''));
      final Dependencies dependencies = await pumpAccount(tester);
      expect(find.text('অতিথি'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('every row is reachable, gaps included', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      PlaceholderScreen? opened;
      int profiles = 0;
      final Dependencies dependencies = await pumpAccount(
        tester,
        onProfile: () => profiles += 1,
        onPlaceholder: (PlaceholderScreen screen) => opened = screen,
      );
      await tester.tap(find.widgetWithText(SettingRow, bn.personalInfo));
      await tester.pumpAndSettle();
      expect(profiles, 1);
      await revealAndTap(
        tester,
        find.widgetWithText(SettingRow, bn.offersTitle),
      );
      expect(opened, PlaceholderScreen.offers);
      dependencies.dispose();
    });

    testWidgets('signing out clears the tokens and the cache', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      route('POST /v1/auth/logout', (_) => null);
      int signedOut = 0;
      final Dependencies dependencies = await pumpAccount(
        tester,
        onSignedOut: () => signedOut += 1,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.signOut),
      );
      expect(signedOut, 1);
      expect(dependencies.session.isSignedIn, isFalse);
      expect(await dependencies.api.cached('/v1/me'), isNull);
      dependencies.dispose();
    });

    testWidgets('a failed server logout still signs out on this handset', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      route('POST /v1/auth/logout', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['POST /v1/auth/logout'] = 503;
      int signedOut = 0;
      final Dependencies dependencies = await pumpAccount(
        tester,
        onSignedOut: () => signedOut += 1,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.signOut),
      );
      expect(signedOut, 1);
      expect(dependencies.session.isSignedIn, isFalse);
      dependencies.dispose();
    });

    testWidgets('a failed profile read offers a retry', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['GET /v1/me'] = 503;
      final Dependencies dependencies = await pumpAccount(tester);
      await tester.tap(find.bySemanticsLabel(core.retry));
      await tester.pumpAndSettle();
      expect(backend.to('/v1/me'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('profile', () {
    Future<Dependencies> pumpProfile(WidgetTester tester) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const ProfileScreen(),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('fills the form and saves only name and email', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me', (_) => profileJson());
      route('PATCH /v1/me', (_) => profileJson(name: 'নাদিয়া'));
      final Dependencies dependencies = await pumpProfile(tester);
      expect(find.text('রিয়া'), findsWidgets);
      await tester.enterText(
        find.widgetWithText(TextField, 'রিয়া').first,
        'নাদিয়া',
      );
      await tester.pumpAndSettle();
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      final Map<String, Object?> body =
          backend.to('/v1/me').last.body! as Map<String, Object?>;
      expect(body.keys.toSet(), <String>{'name', 'email'});
      expect(body['name'], 'নাদিয়া');
      expect(find.text('নাদিয়া'), findsWidgets);
      dependencies.dispose();
    });

    testWidgets('a rejected save is shown', (WidgetTester tester) async {
      route('GET /v1/me', (_) => profileJson());
      route('PATCH /v1/me', (_) => errorBody('invalid_email', 'ইমেইল ভুল'));
      backend.statuses['PATCH /v1/me'] = 400;
      final Dependencies dependencies = await pumpProfile(tester);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(find.text('ইমেইল ভুল'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('addresses', () {
    Future<Dependencies> pumpAddresses(
      WidgetTester tester, {
      Future<bool> Function()? onAdd,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        AddressesScreen(onAdd: onAdd ?? () async => false),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      return dependencies;
    }

    testWidgets('lists the server\'s one-line rendering of each', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      final Dependencies dependencies = await pumpAddresses(tester);
      expect(find.text('রোড ৫, ধানমন্ডি, ঢাকা'), findsOneWidget);
      expect(find.text(bn.defaultAddress), findsOneWidget);
      expect(find.text(bn.makeDefault), findsNothing);
      dependencies.dispose();
    });

    testWidgets('an empty book says so', (WidgetTester tester) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[]});
      final Dependencies dependencies = await pumpAddresses(tester);
      expect(find.text(bn.noAddresses), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('promoting and deleting both re-read the book', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses', (_) => <String, Object?>{
        'addresses': <Object?>[
          addressJson(),
          addressJson(id: 'adr-2', label: 'অফিস', isDefault: false),
        ],
      });
      route('POST /v1/me/addresses/adr-2/default',
          (_) => addressJson(id: 'adr-2'));
      route('DELETE /v1/me/addresses/adr-2', (_) => null);
      final Dependencies dependencies = await pumpAddresses(tester);
      await revealAndTap(tester, find.text(bn.makeDefault));
      expect(backend.to('/v1/me/addresses'), hasLength(2));
      await revealAndTap(tester, find.text(bn.deleteAddress).last);
      expect(backend.to('/v1/me/addresses/adr-2'), hasLength(1));
      expect(backend.to('/v1/me/addresses'), hasLength(3));
      dependencies.dispose();
    });

    testWidgets('a rejected delete is shown', (WidgetTester tester) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      route('DELETE /v1/me/addresses/adr-1',
          (_) => errorBody('in_use', 'চলমান অর্ডারে ব্যবহৃত'));
      backend.statuses['DELETE /v1/me/addresses/adr-1'] = 409;
      final Dependencies dependencies = await pumpAddresses(tester);
      await revealAndTap(tester, find.text(bn.deleteAddress));
      expect(find.text('চলমান অর্ডারে ব্যবহৃত'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('adding one re-reads, and declining does not', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/addresses',
          (_) => <String, Object?>{'addresses': <Object?>[addressJson()]});
      bool added = false;
      final Dependencies dependencies = await pumpAddresses(
        tester,
        onAdd: () async => added,
      );
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addAddress),
      );
      expect(backend.to('/v1/me/addresses'), hasLength(1));
      added = true;
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.addAddress),
      );
      expect(backend.to('/v1/me/addresses'), hasLength(2));
      dependencies.dispose();
    });
  });

  group('add address', () {
    Future<Dependencies> pumpAdd(
      WidgetTester tester, {
      void Function(Address)? onSaved,
    }) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        AddAddressScreen(onSaved: onSaved ?? (_) {}),
        dependencies: dependencies,
      );
      return dependencies;
    }

    Future<void> fill(WidgetTester tester, {String lat = '23.7461'}) async {
      final List<Finder> fields = <Finder>[
        find.widgetWithText(LabelledField, bn.addressLabelField),
        find.widgetWithText(LabelledField, bn.recipientName),
        find.widgetWithText(LabelledField, bn.recipientPhone),
        find.widgetWithText(LabelledField, bn.addressLine1),
      ];
      const List<String> values = <String>[
        'বাসা',
        'রিয়া',
        '01712345678',
        'রোড ৫',
      ];
      for (int i = 0; i < fields.length; i++) {
        await tester.enterText(
          find.descendant(of: fields[i], matching: find.byType(TextField)),
          values[i],
        );
      }
      await tester.enterText(
        find.descendant(
          of: find.widgetWithText(LabelledField, 'lat'),
          matching: find.byType(TextField),
        ),
        lat,
      );
      await tester.enterText(
        find.descendant(
          of: find.widgetWithText(LabelledField, 'lng'),
          matching: find.byType(TextField),
        ),
        '90.3742',
      );
      await tester.pumpAndSettle();
    }

    testWidgets('confirming a point names its area in the server\'s words', (
      WidgetTester tester,
    ) async {
      route('GET /v1/geo/resolve', (_) => areaJson());
      final Dependencies dependencies = await pumpAdd(tester);
      await fill(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.confirm),
      );
      expect(find.text('ধানমন্ডি, ঢাকা'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('a point outside every division is refused by the server', (
      WidgetTester tester,
    ) async {
      route('GET /v1/geo/resolve',
          (_) => errorBody('outside_service_area', 'এই এলাকায় সেবা নেই'));
      backend.statuses['GET /v1/geo/resolve'] = 404;
      final Dependencies dependencies = await pumpAdd(tester);
      await fill(tester);
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.confirm),
      );
      expect(find.text('এই এলাকায় সেবা নেই'), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('saving sends the coordinate and no administrative codes', (
      WidgetTester tester,
    ) async {
      route('POST /v1/me/addresses', (_) => addressJson());
      Address? saved;
      final Dependencies dependencies = await pumpAdd(
        tester,
        onSaved: (Address address) => saved = address,
      );
      await fill(tester);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      final Map<String, Object?> body =
          backend.to('/v1/me/addresses').single.body! as Map<String, Object?>;
      expect(body['lat'], 23.7461);
      expect(body['recipient_name'], 'রিয়া');
      expect(body.containsKey('area_code'), isFalse);
      expect(saved!.id, 'adr-1');
      dependencies.dispose();
    });

    testWidgets('without a usable coordinate nothing is sent', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await pumpAdd(tester);
      await fill(tester, lat: 'not a number');
      await revealAndTap(
        tester,
        find.widgetWithText(GoklayButton, bn.confirm),
      );
      expect(backend.sent, isEmpty);
      expect(
        tester
            .widget<GoklayButton>(
              find.widgetWithText(GoklayButton, bn.save),
            )
            .isEnabled,
        isFalse,
      );
      dependencies.dispose();
    });

    testWidgets('a rejected save shows the server\'s reason', (
      WidgetTester tester,
    ) async {
      route('POST /v1/me/addresses',
          (_) => errorBody('invalid_point', 'অবস্থান ঠিক নয়'));
      backend.statuses['POST /v1/me/addresses'] = 400;
      final Dependencies dependencies = await pumpAdd(tester);
      await fill(tester);
      await revealAndTap(tester, find.widgetWithText(GoklayButton, bn.save));
      expect(find.text('অবস্থান ঠিক নয়'), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('language', () {
    testWidgets('switching changes the requests as well as the words', (
      WidgetTester tester,
    ) async {
      route('PATCH /v1/me', (_) => profileJson());
      final Dependencies dependencies = await harnessDependencies(backend);
      Locale? chosen;
      await pumpScreen(
        tester,
        LanguageScreen(onChanged: (Locale locale) => chosen = locale),
        dependencies: dependencies,
      );
      await tester.tap(find.widgetWithText(SettingRow, bn.english));
      await tester.pumpAndSettle();
      expect(chosen, const Locale('en'));
      expect(dependencies.locale, const Locale('en'));
      expect(backend.to('/v1/me').last.url.queryParameters['lang'], 'en');
      expect(
        (backend.to('/v1/me').last.body! as Map<String, Object?>)['language'],
        'en',
      );

      await tester.tap(find.widgetWithText(SettingRow, bn.bengali));
      await tester.pumpAndSettle();
      expect(dependencies.locale, const Locale('bn'));
      dependencies.dispose();
    });
  });

  group('notifications', () {
    testWidgets('lists what the server composed, marking what never arrived', (
      WidgetTester tester,
    ) async {
      route('GET /v1/me/notifications', (_) => <Object?>[
        notificationJson(),
        notificationJson(status: 'failed'),
      ]);
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const NotificationsScreen(),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text('দোকান আপনার অর্ডার নিয়েছে'), findsNWidgets(2));
      expect(find.text(bn.notificationFailed), findsOneWidget);
      dependencies.dispose();
    });

    testWidgets('an empty history says so', (WidgetTester tester) async {
      route('GET /v1/me/notifications', (_) => <Object?>[]);
      final Dependencies dependencies = await harnessDependencies(backend);
      await pumpScreen(
        tester,
        const NotificationsScreen(),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text(bn.noNotifications), findsOneWidget);
      dependencies.dispose();
    });
  });

  group('security', () {
    testWidgets('lists devices and signs out everywhere', (
      WidgetTester tester,
    ) async {
      route('GET /v1/auth/sessions', (_) => <String, Object?>{
        'sessions': <Object?>[
          <String, Object?>{
            'session_id': 'ses-1',
            'device': 'Pixel 8',
            'current': true,
          },
          <String, Object?>{
            'session_id': 'ses-2',
            'device': 'iPhone 13',
            'current': false,
          },
        ],
      });
      route('POST /v1/auth/logout-all', (_) => null);
      final Dependencies dependencies = await harnessDependencies(backend);
      int signedOut = 0;
      await pumpScreen(
        tester,
        SecurityScreen(onSignedOut: () => signedOut += 1),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      expect(find.text('Pixel 8'), findsOneWidget);
      expect(find.text('iPhone 13'), findsOneWidget);
      await tester.tap(
        find.widgetWithText(GoklayButton, bn.signOutEverywhere),
      );
      await tester.pumpAndSettle();
      expect(signedOut, 1);
      expect(dependencies.session.isSignedIn, isFalse);
      dependencies.dispose();
    });

    testWidgets('a refused revocation keeps the customer signed in', (
      WidgetTester tester,
    ) async {
      route('GET /v1/auth/sessions',
          (_) => <String, Object?>{'sessions': <Object?>[]});
      route('POST /v1/auth/logout-all', (_) => errorBody('boom', 'সমস্যা'));
      backend.statuses['POST /v1/auth/logout-all'] = 503;
      final Dependencies dependencies = await harnessDependencies(backend);
      int signedOut = 0;
      await pumpScreen(
        tester,
        SecurityScreen(onSignedOut: () => signedOut += 1),
        dependencies: dependencies,
      );
      await tester.pumpAndSettle();
      await tester.tap(
        find.widgetWithText(GoklayButton, bn.signOutEverywhere),
      );
      await tester.pumpAndSettle();
      expect(signedOut, 0);
      expect(find.text('সমস্যা'), findsOneWidget);
      expect(dependencies.session.isSignedIn, isTrue);
      dependencies.dispose();
    });
  });

  group('the screens with nothing behind them', () {
    testWidgets('each says so, and two add something true', (
      WidgetTester tester,
    ) async {
      final Dependencies dependencies = await harnessDependencies(backend);
      for (final PlaceholderScreen screen in PlaceholderScreen.values) {
        await pumpScreen(
          tester,
          PlaceholderScreenView(screen: screen),
          dependencies: dependencies,
        );
        expect(
          find.text(PlaceholderScreenView.titleOf(bn, screen)),
          findsOneWidget,
        );
        expect(find.text(core.notAvailableYet), findsOneWidget);
        expect(find.byType(NotAvailableView), findsOneWidget);
      }
      expect(
        PlaceholderScreenView.detailOf(bn, PlaceholderScreen.paymentMethods),
        bn.paymentMethodsBody,
      );
      expect(
        PlaceholderScreenView.detailOf(bn, PlaceholderScreen.permissions),
        contains(bn.permissionLocation),
      );
      expect(
        PlaceholderScreenView.detailOf(bn, PlaceholderScreen.offers),
        isNull,
      );
      dependencies.dispose();
    });
  });
}
