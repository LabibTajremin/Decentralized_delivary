import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:goklay_core/goklay_core.dart';
import 'package:http/http.dart' as http;

import 'api/models/auth.dart';
import 'app_scope.dart';
import 'dependencies.dart';
import 'environment.dart';
import 'l10n/customer_strings.dart';
import 'screens/main_shell.dart';
import 'screens/onboarding_screen.dart';
import 'screens/otp_screen.dart';
import 'screens/phone_sign_in_screen.dart';
import 'screens/sign_in_options_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/verification_result_screen.dart';

/// The customer app.
///
/// It owns four things and delegates everything else: the dependency graph,
/// the theme (from `goklay_core`, so all three apps look like one product),
/// the localisation delegates (Bengali first), and the language the whole
/// tree is currently in.
class GoklayCustomerApp extends StatefulWidget {
  /// Creates the app against [environment].
  ///
  /// [dependencies] is injected by tests, which pass a graph built on a fake
  /// HTTP client. In production it is built here, from the environment.
  const GoklayCustomerApp({
    required this.environment,
    this.dependencies,
    this.home,
    super.key,
  });

  /// Where this build points.
  final CustomerEnvironment environment;

  /// The graph to use, or null to build one.
  final Dependencies? dependencies;

  /// The root screen. Tests pass one directly rather than driving the whole
  /// app to reach the screen under test.
  final Widget? home;

  @override
  State<GoklayCustomerApp> createState() => _GoklayCustomerAppState();
}

class _GoklayCustomerAppState extends State<GoklayCustomerApp> {
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
    return AppScope(
      dependencies: _dependencies,
      child: MaterialApp(
        title: 'GoKlay',
        debugShowCheckedModeBanner: false,
        theme: GoklayTheme.light(),
        locale: _locale,
        localizationsDelegates: const <LocalizationsDelegate<Object>>[
          GoklayLocalizations.delegate,
          CustomerLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        supportedLocales: GoklayLocalizations.supportedLocales,
        home:
            widget.home ??
            CustomerRoot(onLanguageChanged: _changeLanguage),
      ),
    );
  }
}

/// Which of the app's two halves is showing: signing in, or signed in.
enum _Stage {
  /// Reading stored tokens.
  splash,

  /// The three onboarding pages.
  onboarding,

  /// The one sign-in method.
  signInOptions,

  /// Entering a phone number.
  phone,

  /// Entering the code.
  otp,

  /// The code was accepted.
  verified,

  /// The code was refused.
  failed,

  /// The tabs.
  shell,
}

/// The sign-in flow and the app behind it.
///
/// Kept in one widget because it is one decision — is anybody signed in —
/// taken in two places: once at launch, and again whenever somebody signs out.
/// Splitting it across routes would mean two answers to keep in step.
class CustomerRoot extends StatefulWidget {
  /// Creates the root.
  const CustomerRoot({required this.onLanguageChanged, super.key});

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  @override
  State<CustomerRoot> createState() => _CustomerRootState();
}

class _CustomerRootState extends State<CustomerRoot> {
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
    return switch (_stage) {
      _Stage.splash => SplashScreen(
        onReady: (bool isSignedIn) =>
            _to(isSignedIn ? _Stage.shell : _Stage.onboarding),
      ),
      _Stage.onboarding => OnboardingScreen(
        onFinished: () => _to(_Stage.signInOptions),
      ),
      _Stage.signInOptions => SignInOptionsScreen(
        onContinue: () => _to(_Stage.phone),
      ),
      _Stage.phone => PhoneSignInScreen(
        onCodeSent: (String phone, OtpChallenge challenge) => setState(() {
          _phone = phone;
          _challenge = challenge;
          _stage = _Stage.otp;
        }),
      ),
      _Stage.otp => OtpScreen(
        phone: _phone,
        challenge: _challenge,
        onVerified: (AuthResult _) => _to(_Stage.verified),
        onFailed: (ApiError error) => setState(() {
          _failure = error;
          _stage = _Stage.failed;
        }),
      ),
      _Stage.verified => VerificationResultScreen.success(
        onContinue: () => _to(_Stage.shell),
      ),
      _Stage.failed => VerificationResultScreen.failure(
        error: _failure,
        onContinue: () => _to(_Stage.phone),
      ),
      _Stage.shell => MainShell(
        onSignedOut: () => _to(_Stage.signInOptions),
        onLanguageChanged: widget.onLanguageChanged,
      ),
    };
  }
}
