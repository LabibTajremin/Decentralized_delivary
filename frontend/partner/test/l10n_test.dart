import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_partner/src/l10n/partner_strings.dart';

/// Every getter on the table, so a string added to one language and forgotten
/// in the other cannot pass.
List<String> allStrings(PartnerStrings s) => <String>[
  s.languageTag,
  s.registerTitle,
  s.registerSubtitle,
  s.riderName,
  s.riderPhone,
  s.vehicle,
  s.register,
  s.goOnShift,
  s.goOffShift,
  s.preference,
  s.preferenceShort,
  s.preferenceLong,
  s.preferenceAny,
  s.yourLocation,
  s.updateLocation,
  s.acceptance,
  s.feedTitle,
  s.acceptJob,
  s.declineJob,
  s.jobsTitle,
  s.jobsLive,
  s.jobsPast,
  s.noJobs,
  s.pickup,
  s.dropOff,
  s.collect,
  s.deliver,
  s.failJob,
  s.failReason,
  s.toPickup,
  s.wholeDistance,
  s.cashTitle,
  s.outstanding,
  s.remitted,
  s.heldCollections,
  s.settled,
  s.waitingToSend,
  s.sendNow,
  s.queuedActionRefused,
  s.accountTitle,
  s.signOut,
];

/// The Bengali block, `U+0980..U+09FF`.
bool hasBengali(String value) =>
    value.runes.any((int rune) => rune >= 0x0980 && rune <= 0x09FF);

void main() {
  const PartnerStringsBn bn = PartnerStringsBn();
  const PartnerStringsEn en = PartnerStringsEn();

  test('both tables answer every string', () {
    expect(allStrings(bn), hasLength(allStrings(en).length));
    expect(allStrings(bn).where((String s) => s.isEmpty), isEmpty);
    expect(allStrings(en).where((String s) => s.isEmpty), isEmpty);
  });

  test('the Bengali table is actually in Bengali script', () {
    // The language tag is a code; everything else a rider reads is Bengali.
    final Iterable<String> latinOnly = allStrings(
      bn,
    ).where((String s) => s != bn.languageTag && !hasBengali(s));
    expect(latinOnly, isEmpty, reason: 'not in Bengali: $latinOnly');
  });

  test('the English table is not Bengali', () {
    expect(allStrings(en).where(hasBengali), isEmpty);
  });

  test('anything that is not English resolves to Bengali', () {
    expect(
      PartnerLocalizations.forLocale(const Locale('en')).languageTag,
      'en',
    );
    expect(
      PartnerLocalizations.forLocale(const Locale('bn')).languageTag,
      'bn',
    );
    expect(
      PartnerLocalizations.forLocale(const Locale('ar')).languageTag,
      'bn',
    );
  });

  test('the delegate supports the two locales and reloads for neither', () {
    const LocalizationsDelegate<PartnerStrings> delegate =
        PartnerLocalizations.delegate;
    expect(delegate.isSupported(const Locale('bn')), isTrue);
    expect(delegate.isSupported(const Locale('en')), isTrue);
    expect(delegate.isSupported(const Locale('ar')), isFalse);
    expect(delegate.shouldReload(delegate), isFalse);
  });

  testWidgets('outside a MaterialApp the table falls back to Bengali', (
    WidgetTester tester,
  ) async {
    late PartnerStrings seen;
    await tester.pumpWidget(
      Builder(
        builder: (BuildContext context) {
          seen = PartnerLocalizations.of(context);
          return const SizedBox.shrink();
        },
      ),
    );
    expect(seen.languageTag, 'bn');
  });

  testWidgets('inside one it is the app\'s locale', (
    WidgetTester tester,
  ) async {
    late PartnerStrings seen;
    await tester.pumpWidget(
      MaterialApp(
        locale: const Locale('en'),
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          PartnerLocalizations.delegate,
        ],
        supportedLocales: const <Locale>[Locale('bn'), Locale('en')],
        home: Builder(
          builder: (BuildContext context) {
            seen = PartnerLocalizations.of(context);
            return const SizedBox.shrink();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(seen.languageTag, 'en');
  });
}
