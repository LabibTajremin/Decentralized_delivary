import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/models/merchant.dart';
import 'app_scope.dart';
import 'dependencies.dart';
import 'environment.dart';
import 'l10n/merchant_strings.dart';
import 'screens/main_shell.dart';
import 'screens/shop_details_screen.dart';

/// The merchant app.
class GoklayMerchantApp extends StatefulWidget {
  /// Creates the app against [environment].
  const GoklayMerchantApp({
    required this.environment,
    this.dependencies,
    this.home,
    super.key,
  });

  /// Where this build points.
  final MerchantEnvironment environment;

  /// The graph to use, or null to build one.
  final Dependencies? dependencies;

  /// The root screen. Tests pass one directly.
  final Widget? home;

  @override
  State<GoklayMerchantApp> createState() => _GoklayMerchantAppState();
}

class _GoklayMerchantAppState extends State<GoklayMerchantApp> {
  late final Dependencies _dependencies =
      widget.dependencies ??
      Dependencies(
        environment: widget.environment,
        httpClient: http.Client(),
      );
  late Locale _locale = _dependencies.locale;

  void _changeLanguage(Locale locale) {
    _dependencies.locale = locale;
    setState(() => _locale = locale);
  }

  @override
  Widget build(BuildContext context) {
    return MerchantScope(
      dependencies: _dependencies,
      child: MaterialApp(
        title: 'GoKlay Merchant',
        debugShowCheckedModeBanner: false,
        theme: GoklayTheme.light(),
        locale: _locale,
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          GoklayLocalizations.delegate,
          MerchantLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: GoklayLocalizations.supportedLocales,
        home:
            widget.home ??
            MerchantRoot(onLanguageChanged: _changeLanguage),
      ),
    );
  }
}

/// Which half of the app is showing.
enum _Stage {
  /// Reading stored tokens.
  splash,

  /// Entering a phone number.
  phone,

  /// Entering the code.
  otp,

  /// The code was refused.
  failed,

  /// Signed in: find out whether this owner has a shop yet.
  shop,
}

/// The sign-in flow, and behind it the question every merchant session opens
/// with: does this owner have a shop yet?
class MerchantRoot extends StatefulWidget {
  /// Creates the root.
  const MerchantRoot({required this.onLanguageChanged, super.key});

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<MerchantRoot> createState() => _MerchantRootState();
}

class _MerchantRootState extends State<MerchantRoot> {
  _Stage _stage = _Stage.splash;
  String _phone = '';
  OtpChallenge _challenge = const OtpChallenge(
    expiresIn: Duration.zero,
    resendAfter: Duration.zero,
  );
  ApiError? _failure;

  void _to(_Stage stage) => setState(() => _stage = stage);

  @override
  Widget build(BuildContext context) {
    final Dependencies dependencies = MerchantScope.of(context);
    return switch (_stage) {
      _Stage.splash => GoklaySplashScreen(
        session: dependencies.session,
        onReady: (bool isSignedIn) =>
            _to(isSignedIn ? _Stage.shop : _Stage.phone),
      ),
      _Stage.phone => PhoneSignInScreen(
        auth: dependencies.auth,
        onCodeSent: (String phone, OtpChallenge challenge) => setState(() {
          _phone = phone;
          _challenge = challenge;
          _stage = _Stage.otp;
        }),
      ),
      _Stage.otp => OtpScreen(
        auth: dependencies.auth,
        session: dependencies.session,
        role: AuthResult.merchantRole,
        phone: _phone,
        challenge: _challenge,
        onVerified: (AuthResult _) => _to(_Stage.shop),
        onFailed: (ApiError error) => setState(() {
          _failure = error;
          _stage = _Stage.failed;
        }),
      ),
      _Stage.failed => VerificationResultScreen.failure(
        error: _failure,
        onContinue: () => _to(_Stage.phone),
      ),
      _Stage.shop => ShopGate(
        onSignedOut: () => _to(_Stage.phone),
        onLanguageChanged: widget.onLanguageChanged,
      ),
    };
  }
}

/// Decides whether this owner is registering a shop or running one.
///
/// `GET /v1/merchants/me` answers 404 when the owner has no shop, which is
/// how the app knows to show the registration form — rather than keeping a
/// local "have I registered yet" flag that a reinstall would lose and a
/// second device would disagree with.
class ShopGate extends StatefulWidget {
  /// Creates the gate.
  const ShopGate({
    required this.onSignedOut,
    required this.onLanguageChanged,
    super.key,
  });

  /// Called when the owner signs out.
  final VoidCallback onSignedOut;

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<ShopGate> createState() => _ShopGateState();
}

class _ShopGateState extends State<ShopGate> {
  final Store<Merchant?> _shop = Store<Merchant?>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _shop.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _shop.load(() async {
      try {
        final ApiPage<Merchant> page = await dependencies.merchant.me();
        return AsyncData<Merchant?>(
          page.value,
          fromCache: page.fromCache,
          storedAt: page.storedAt,
        );
      } on ApiError catch (error) {
        if (error.statusCode == 404) {
          return const AsyncData<Merchant?>(null);
        }
        rethrow;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return AsyncView<Merchant?>(
      store: _shop,
      onRetry: _load,
      builder: (BuildContext context, Merchant? shop) => shop == null
          ? ShopDetailsScreen(
              onSaved: (Merchant saved) =>
                  _shop.emit(AsyncData<Merchant?>(saved)),
            )
          : MerchantShell(
              merchantId: shop.id,
              onSignedOut: widget.onSignedOut,
              onLanguageChanged: widget.onLanguageChanged,
            ),
    );
  }
}
