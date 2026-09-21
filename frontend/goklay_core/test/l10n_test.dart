import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:goklay_core/goklay_core.dart';

/// The narrowest screen this product realistically runs on. The design is
/// drawn at 430; a budget Android of the kind rural users actually carry is
/// 320. Bengali has to fit both.
const double kNarrowPhone = 320;

Widget _app({required Locale locale, required Widget child}) {
  return MaterialApp(
    locale: locale,
    theme: GoklayTheme.light(),
    localizationsDelegates: const <LocalizationsDelegate<Object>>[
      GoklayLocalizations.delegate,
      GlobalMaterialLocalizations.delegate,
      GlobalWidgetsLocalizations.delegate,
      GlobalCupertinoLocalizations.delegate,
    ],
    supportedLocales: GoklayLocalizations.supportedLocales,
    home: Scaffold(body: Center(child: child)),
  );
}

void main() {
  group('the tables', () {
    test('Bengali is what an unrecognised locale falls back to', () {
      expect(GoklayLocalizations.supportedLocales.first.languageCode, 'bn');
      expect(
        GoklayLocalizations.forLocale(const Locale('hi')).languageTag,
        'bn',
      );
      expect(
        GoklayLocalizations.forLocale(const Locale('ar')).languageTag,
        'bn',
      );
      expect(
        GoklayLocalizations.forLocale(const Locale('en')).languageTag,
        'en',
      );
    });

    test('every string is present in both languages, and they differ', () {
      const GoklayStrings bn = GoklayStringsBn();
      const GoklayStrings en = GoklayStringsEn();

      final List<(String, String)> pairs = <(String, String)>[
        (bn.retry, en.retry),
        (bn.cancel, en.cancel),
        (bn.close, en.close),
        (bn.loading, en.loading),
        (bn.offlineBanner, en.offlineBanner),
        (bn.queuedOffline, en.queuedOffline),
        (bn.unexpectedError, en.unexpectedError),
        // P19 moved five more here, when the widgets that say them moved out
        // of the customer app into this package.
        (bn.showingSaved, en.showingSaved),
        (bn.nothingHere, en.nothingHere),
        (bn.notAvailableYet, en.notAvailableYet),
        (bn.decreaseQuantity, en.decreaseQuantity),
        (bn.increaseQuantity, en.increaseQuantity),
        // And fifteen more when the sign-in flow itself moved here, because
        // all three apps have it and only the role differs.
        (bn.signInTitle, en.signInTitle),
        (bn.phoneLabel, en.phoneLabel),
        (bn.sendCode, en.sendCode),
        (bn.otpTitle, en.otpTitle),
        (bn.otpSubtitle, en.otpSubtitle),
        (bn.verifyCode, en.verifyCode),
        (bn.resendCountdown, en.resendCountdown),
        (bn.resendCode, en.resendCode),
        (bn.verifiedTitle, en.verifiedTitle),
        (bn.verifiedBody, en.verifiedBody),
        (bn.verificationFailedTitle, en.verificationFailedTitle),
        (bn.verificationFailedBody, en.verificationFailedBody),
        (bn.startOver, en.startOver),
        (bn.getStarted, en.getStarted),
      ];

      // `phoneHint` is deliberately not in the pair list: it is a number
      // format, `01XXXXXXXXX`, and is the same in both languages.
      expect(bn.phoneHint, en.phoneHint);

      for (final (String b, String e) in pairs) {
        expect(b, isNotEmpty);
        expect(e, isNotEmpty);
        expect(
          b,
          isNot(e),
          reason: 'an untranslated string is an English string in disguise',
        );
      }
    });

    test('the Bengali table is actually in Bengali script', () {
      // A translation that quietly stayed in English passes a "not empty"
      // check. It does not pass this one: U+0980..U+09FF is the Bengali block.
      const GoklayStrings bn = GoklayStringsBn();
      for (final String value in <String>[
        bn.retry,
        bn.cancel,
        bn.close,
        bn.loading,
        bn.offlineBanner,
        bn.queuedOffline,
        bn.unexpectedError,
        bn.showingSaved,
        bn.nothingHere,
        bn.notAvailableYet,
        bn.decreaseQuantity,
        bn.increaseQuantity,
        bn.signInTitle,
        bn.phoneLabel,
        bn.sendCode,
        bn.otpTitle,
        bn.otpSubtitle,
        bn.verifyCode,
        bn.resendCountdown,
        bn.resendCode,
        bn.verifiedTitle,
        bn.verifiedBody,
        bn.verificationFailedTitle,
        bn.verificationFailedBody,
        bn.startOver,
        bn.getStarted,
      ]) {
        expect(
          value.runes.any((int r) => r >= 0x0980 && r <= 0x09FF),
          isTrue,
          reason: '"$value" contains no Bengali characters',
        );
      }
    });
  });

  group('the lang query parameter', () {
    test('is sent only for English, because the server is Bengali-first', () {
      expect(
        GoklayLocalizations.languageQueryValue(const Locale('bn')),
        isNull,
      );
      expect(
        GoklayLocalizations.languageQueryValue(const Locale('hi')),
        isNull,
      );
      expect(GoklayLocalizations.languageQueryValue(const Locale('en')), 'en');
    });
  });

  group('the delegate', () {
    test('supports exactly the two languages, and reloads for neither', () {
      const LocalizationsDelegate<GoklayStrings> delegate =
          GoklayLocalizations.delegate;
      expect(delegate.isSupported(const Locale('bn')), isTrue);
      expect(delegate.isSupported(const Locale('en')), isTrue);
      expect(delegate.isSupported(const Locale('fr')), isFalse);
      expect(delegate.shouldReload(delegate), isFalse);
    });

    test('loads the table for the locale it is asked for', () async {
      expect(
        (await GoklayLocalizations.delegate.load(const Locale('en')))
            .languageTag,
        'en',
      );
      expect(
        (await GoklayLocalizations.delegate.load(const Locale('bn')))
            .languageTag,
        'bn',
      );
    });

    testWidgets('of() returns the table the delegate installed', (
      WidgetTester tester,
    ) async {
      late GoklayStrings seen;
      await tester.pumpWidget(
        _app(
          locale: const Locale('en'),
          child: Builder(
            builder: (BuildContext context) {
              seen = GoklayLocalizations.of(context);
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(seen.languageTag, 'en');
    });

    testWidgets('of() falls back to Bengali when no delegate is installed', (
      WidgetTester tester,
    ) async {
      // A screen must not throw because somebody forgot a delegate.
      late GoklayStrings seen;
      await tester.pumpWidget(
        Directionality(
          textDirection: TextDirection.ltr,
          child: Builder(
            builder: (BuildContext context) {
              seen = GoklayLocalizations.of(context);
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      expect(seen.languageTag, 'bn');
    });
  });

  group('Bengali renders at Bengali lengths', () {
    testWidgets('the default locale paints the Bengali table', (
      WidgetTester tester,
    ) async {
      await tester.pumpWidget(
        _app(
          locale: const Locale('bn'),
          child: Builder(
            builder: (BuildContext context) => GoklayButton(
              label: GoklayLocalizations.of(context).retry,
              onPressed: () {},
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(const GoklayStringsBn().retry), findsOneWidget);
    });

    testWidgets('a button fits its Bengali label on a 320dp screen', (
      WidgetTester tester,
    ) async {
      tester.view.physicalSize = const Size(kNarrowPhone, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        _app(
          locale: const Locale('bn'),
          child: Builder(
            builder: (BuildContext context) {
              final GoklayStrings strings = GoklayLocalizations.of(context);
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  GoklayButton(label: strings.retry, onPressed: () {}),
                  GoklayButton(
                    label: strings.cancel,
                    onPressed: () {},
                    variant: GoklayButtonVariant.outlined,
                  ),
                ],
              );
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      // A RenderFlex overflow surfaces here. Bengali is the longest of the two
      // languages, so if it fits, English does.
      expect(tester.takeException(), isNull);
    });

    testWidgets('the longest Bengali sentence wraps rather than overflowing', (
      WidgetTester tester,
    ) async {
      tester.view.physicalSize = const Size(kNarrowPhone, 640);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        _app(
          locale: const Locale('bn'),
          child: Builder(
            builder: (BuildContext context) => Padding(
              padding: const EdgeInsets.all(GoklaySpacing.lg),
              child: Text(
                GoklayLocalizations.of(context).unexpectedError,
                style: GoklayTextStyles.body,
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(
        find.text(const GoklayStringsBn().unexpectedError),
        findsOneWidget,
      );
    });

    test('Bengali is the longer language, which is why layout is tested', () {
      const GoklayStrings bn = GoklayStringsBn();
      const GoklayStrings en = GoklayStringsEn();
      // Not a rule to enforce — a fact that explains why every layout in this
      // codebase is measured in Bengali and never in English.
      expect(bn.retry.length, greaterThan(en.retry.length));
    });
  });
}
