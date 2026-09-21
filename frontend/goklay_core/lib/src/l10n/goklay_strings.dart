import 'package:flutter/widgets.dart';

/// The client's own string table — and it is deliberately this short.
///
/// Every sentence a user reads about their order, their cart, their delivery
/// or their money is composed by the server and arrives ready to paint (2.9).
/// What is left for the client is the handful of words that have to exist
/// *before* a server answers, or when one never does: a retry button, an
/// offline banner, the fallback for an error with no message.
///
/// That is why this is written by hand rather than generated from ARB files.
/// Codegen earns its keep across hundreds of strings; across eight it buys
/// nothing and costs a generated file that the 100% coverage gate would then
/// have to be taught to ignore.
///
/// Adding a string here deserves a second look. If it describes anything about
/// an order, a price, a shop or a rule, it belongs in an API response instead.
abstract class GoklayStrings {
  /// The label on the button that repeats a failed request.
  String get retry;

  /// The label that abandons whatever is open.
  String get cancel;

  /// The label that closes a sheet or dialog.
  String get close;

  /// Shown while a screen is waiting for its first response.
  String get loading;

  /// The banner shown when the device has no connection.
  String get offlineBanner;

  /// Shown when an action was taken offline and is waiting to be sent.
  String get queuedOffline;

  /// The fallback when a failure arrives carrying no message of its own.
  /// A server-supplied message is always preferred over this.
  String get unexpectedError;

  /// The language tag this table is written in.
  String get languageTag;
}

/// Bengali. The default: this product is Bengali-first (1.4), so this is what
/// a user sees unless they have explicitly chosen English.
class GoklayStringsBn implements GoklayStrings {
  /// Creates the Bengali string table.
  const GoklayStringsBn();

  @override
  String get retry => 'আবার চেষ্টা করুন';

  @override
  String get cancel => 'বাতিল করুন';

  @override
  String get close => 'বন্ধ করুন';

  @override
  String get loading => 'লোড হচ্ছে…';

  @override
  String get offlineBanner => 'ইন্টারনেট সংযোগ নেই';

  @override
  String get queuedOffline => 'সংযোগ ফিরে পেলে পাঠানো হবে';

  @override
  String get unexpectedError => 'কিছু একটা ভুল হয়েছে। আবার চেষ্টা করুন।';

  @override
  String get languageTag => 'bn';
}

/// English. The exception, reached only when the user asks for it.
class GoklayStringsEn implements GoklayStrings {
  /// Creates the English string table.
  const GoklayStringsEn();

  @override
  String get retry => 'Try again';

  @override
  String get cancel => 'Cancel';

  @override
  String get close => 'Close';

  @override
  String get loading => 'Loading…';

  @override
  String get offlineBanner => 'No internet connection';

  @override
  String get queuedOffline => 'Will be sent when you are back online';

  @override
  String get unexpectedError => 'Something went wrong. Please try again.';

  @override
  String get languageTag => 'en';
}

/// Wires [GoklayStrings] into the widget tree, Bengali first.
abstract final class GoklayLocalizations {
  /// Bengali leads the list on purpose. Flutter resolves an unmatched device
  /// locale to the first supported one, so a phone set to Hindi, Arabic or
  /// anything else this app has never heard of lands on Bengali rather than
  /// English.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('bn'),
    Locale('en'),
  ];

  /// The delegate to hand to `MaterialApp.localizationsDelegates`.
  static const LocalizationsDelegate<GoklayStrings> delegate =
      _GoklayStringsDelegate();

  /// The table for [locale]. Anything that is not English is Bengali.
  static GoklayStrings forLocale(Locale locale) {
    return locale.languageCode == 'en'
        ? const GoklayStringsEn()
        : const GoklayStringsBn();
  }

  /// The table in scope, or Bengali when the delegate was never installed —
  /// a missing delegate must not be the reason a screen throws.
  static GoklayStrings of(BuildContext context) {
    return Localizations.of<GoklayStrings>(context, GoklayStrings) ??
        const GoklayStringsBn();
  }

  /// The value for the API's `lang` query parameter, or null for Bengali.
  ///
  /// The server is Bengali-first too: it answers in Bengali unless asked
  /// otherwise, so the parameter is sent only for English. Sending
  /// `lang=bn` everywhere would work and would also be noise on every URL.
  static String? languageQueryValue(Locale locale) =>
      locale.languageCode == 'en' ? 'en' : null;
}

class _GoklayStringsDelegate extends LocalizationsDelegate<GoklayStrings> {
  const _GoklayStringsDelegate();

  @override
  bool isSupported(Locale locale) =>
      GoklayLocalizations.supportedLocales.any(
        (Locale l) => l.languageCode == locale.languageCode,
      );

  @override
  Future<GoklayStrings> load(Locale locale) async =>
      GoklayLocalizations.forLocale(locale);

  @override
  bool shouldReload(_GoklayStringsDelegate old) => false;
}
