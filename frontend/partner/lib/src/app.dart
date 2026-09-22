import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/models/partner.dart';
import 'app_scope.dart';
import 'dependencies.dart';
import 'environment.dart';
import 'l10n/partner_strings.dart';
import 'screens/main_shell.dart';
import 'screens/register_screen.dart';

/// The partner app.
class GoklayPartnerApp extends StatefulWidget {
  /// Creates the app against [environment].
  const GoklayPartnerApp({
    required this.environment,
    this.dependencies,
    this.home,
    super.key,
  });

  /// Where this build points.
  final PartnerEnvironment environment;

  /// The graph to use, or null to build one.
  final Dependencies? dependencies;

  /// The root screen. Tests pass one directly.
  final Widget? home;

  @override
  State<GoklayPartnerApp> createState() => _GoklayPartnerAppState();
}

class _GoklayPartnerAppState extends State<GoklayPartnerApp> {
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
    return PartnerScope(
      dependencies: _dependencies,
      child: MaterialApp(
        title: 'GoKlay Partner',
        debugShowCheckedModeBanner: false,
        theme: GoklayTheme.light(),
        locale: _locale,
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          GoklayLocalizations.delegate,
          PartnerLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: GoklayLocalizations.supportedLocales,
        home:
            widget.home ?? PartnerRoot(onLanguageChanged: _changeLanguage),
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

  /// Signed in: find out whether this rider has signed up yet.
  partner,
}

/// The sign-in flow, and behind it the question every rider session opens
/// with: has this account signed up to carry orders?
class PartnerRoot extends StatefulWidget {
  /// Creates the root.
  const PartnerRoot({required this.onLanguageChanged, super.key});

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<PartnerRoot> createState() => _PartnerRootState();
}

class _PartnerRootState extends State<PartnerRoot> {
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
    final Dependencies dependencies = PartnerScope.of(context);
    return switch (_stage) {
      _Stage.splash => GoklaySplashScreen(
        session: dependencies.session,
        onReady: (bool isSignedIn) =>
            _to(isSignedIn ? _Stage.partner : _Stage.phone),
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
        role: AuthResult.partnerRole,
        phone: _phone,
        challenge: _challenge,
        onVerified: (AuthResult _) => _to(_Stage.partner),
        onFailed: (ApiError error) => setState(() {
          _failure = error;
          _stage = _Stage.failed;
        }),
      ),
      _Stage.failed => VerificationResultScreen.failure(
        error: _failure,
        onContinue: () => _to(_Stage.phone),
      ),
      _Stage.partner => PartnerGate(
        onSignedOut: () => _to(_Stage.phone),
        onLanguageChanged: widget.onLanguageChanged,
      ),
    };
  }
}

/// Decides whether this account is signing up or already riding.
///
/// `GET /v1/partner` answers 404 for an account that has not signed up, which
/// is how the app knows to show the form — rather than keeping a local flag
/// that a reinstall would lose and a second handset would disagree with.
class PartnerGate extends StatefulWidget {
  /// Creates the gate.
  const PartnerGate({
    required this.onSignedOut,
    required this.onLanguageChanged,
    super.key,
  });

  /// Called when the rider signs out.
  final VoidCallback onSignedOut;

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<PartnerGate> createState() => _PartnerGateState();
}

class _PartnerGateState extends State<PartnerGate> {
  final Store<DeliveryPartner?> _partner = Store<DeliveryPartner?>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _partner.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _partner.load(() async {
      try {
        final ApiPage<DeliveryPartner> page = await dependencies.partner.me();
        return AsyncData<DeliveryPartner?>(
          page.value,
          fromCache: page.fromCache,
          storedAt: page.storedAt,
        );
      } on ApiError catch (error) {
        if (error.statusCode == 404) {
          return const AsyncData<DeliveryPartner?>(null);
        }
        rethrow;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return AsyncView<DeliveryPartner?>(
      store: _partner,
      onRetry: _load,
      builder: (BuildContext context, DeliveryPartner? partner) =>
          partner == null
          ? RegisterScreen(
              onRegistered: (DeliveryPartner saved) =>
                  _partner.emit(AsyncData<DeliveryPartner?>(saved)),
            )
          : PartnerShell(
              onSignedOut: widget.onSignedOut,
              onLanguageChanged: widget.onLanguageChanged,
            ),
    );
  }
}
