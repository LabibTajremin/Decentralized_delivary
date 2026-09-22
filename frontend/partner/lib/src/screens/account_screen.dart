import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/partner.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/partner_strings.dart';
import '../widgets/cards.dart';

/// The rider's own settings: their distance choice, their reported position,
/// and the way out.
///
/// The three distance choices are D4's, and `any` is the default — a partner
/// who has not chosen has not chosen to exclude anything. The label beside
/// each is the server's sentence, so the app never explains the rule itself.
class PartnerAccountScreen extends StatefulWidget {
  /// Creates the screen.
  const PartnerAccountScreen({
    required this.onLanguageChanged,
    required this.onSignedOut,
    this.showBack = true,
    super.key,
  });

  /// Called when the language changes.
  final void Function(Locale locale) onLanguageChanged;

  /// Called once the tokens are cleared.
  final VoidCallback onSignedOut;

  /// Whether to draw a back button.
  final bool showBack;

  @override
  State<PartnerAccountScreen> createState() => _PartnerAccountScreenState();
}

class _PartnerAccountScreenState extends State<PartnerAccountScreen> {
  final Store<DeliveryPartner> _partner = Store<DeliveryPartner>();
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _lat = TextEditingController();
  final TextEditingController _lng = TextEditingController();

  @override
  void initState() {
    super.initState();
    _lat.addListener(_onTyped);
    _lng.addListener(_onTyped);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _lat
      ..removeListener(_onTyped)
      ..dispose();
    _lng
      ..removeListener(_onTyped)
      ..dispose();
    _partner.dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  (double, double)? get _point {
    final double? lat = double.tryParse(_lat.text.trim());
    final double? lng = double.tryParse(_lng.text.trim());
    return lat == null || lng == null ? null : (lat, lng);
  }

  Future<void> _load() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _partner.load(() async {
      final ApiPage<DeliveryPartner> page = await dependencies.partner.me();
      final DeliveryPartner partner = page.value;
      _lat.text = '${partner.lat}';
      _lng.text = '${partner.lng}';
      return page.toAsyncData();
    });
  }

  Future<void> _apply(Future<DeliveryPartner> Function() call) async {
    await _runner.run(() async {
      final DeliveryPartner updated = await call();
      _partner.emit(AsyncData<DeliveryPartner>(updated));
    });
  }

  Future<void> _report() async {
    final (double, double)? point = _point;
    if (point == null) {
      return;
    }
    final Dependencies dependencies = PartnerScope.of(context);
    await _apply(
      () => dependencies.partner.reportLocation(
        lat: point.$1,
        lng: point.$2,
      ),
    );
  }

  Future<void> _signOut() async {
    final Dependencies dependencies = PartnerScope.of(context);
    await _runner.run(() async {
      try {
        await dependencies.auth.logout();
      } on ApiError {
        // A rider on a handset with no signal must still end up signed out.
      }
      await dependencies.signOut();
    });
    if (mounted) {
      widget.onSignedOut();
    }
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.accountTitle,
      showBack: widget.showBack,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => AsyncView<DeliveryPartner>(
          store: _partner,
          onRetry: _load,
          builder: (BuildContext context, DeliveryPartner partner) =>
              _body(strings, partner),
        ),
      ),
    );
  }

  Widget _body(PartnerStrings strings, DeliveryPartner partner) {
    final Dependencies dependencies = PartnerScope.of(context);
    return ListView(
      children: <Widget>[
        GoklayCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(partner.name, style: GoklayTextStyles.title),
              Text(partner.phone, style: GoklayTextStyles.body),
              Text(
                partner.availabilityLabel,
                style: GoklayTextStyles.caption.copyWith(
                  color: GoklayColors.textSecondary,
                ),
              ),
              Text(
                '${strings.acceptance} ${partner.acceptancePercent}%',
                style: GoklayTextStyles.caption,
              ),
            ],
          ),
        ),
        SectionHeading(strings.preference),
        for (final (String value, String label) in <(String, String)>[
          ('short', strings.preferenceShort),
          ('long', strings.preferenceLong),
          ('any', strings.preferenceAny),
        ])
          SettingRow(
            label: label,
            value: partner.preference == value ? '✓' : null,
            onTap: _runner.isBusy
                ? null
                : () => _apply(
                    () => dependencies.partner.setPreference(value),
                  ),
          ),
        SectionHeading(strings.yourLocation),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: GoklaySpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Expanded(
                    child: LabelledField(
                      label: 'lat',
                      controller: _lat,
                      keyboardType: TextInputType.number,
                    ),
                  ),
                  const SizedBox(width: GoklaySpacing.md),
                  Expanded(
                    child: LabelledField(
                      label: 'lng',
                      controller: _lng,
                      keyboardType: TextInputType.number,
                    ),
                  ),
                ],
              ),
              GoklayButton(
                label: strings.updateLocation,
                variant: GoklayButtonVariant.outlined,
                expand: true,
                onPressed: _runner.isBusy || _point == null ? null : _report,
              ),
            ],
          ),
        ),
        SectionHeading('ভাষা / Language'),
        SettingRow(
          label: 'বাংলা',
          value: strings.languageTag == 'bn' ? '✓' : null,
          onTap: _runner.isBusy ? null : () => _setLanguage('bn'),
        ),
        SettingRow(
          label: 'English',
          value: strings.languageTag == 'en' ? '✓' : null,
          onTap: _runner.isBusy ? null : () => _setLanguage('en'),
        ),
        if (_runner.error != null)
          Padding(
            padding: const EdgeInsets.all(GoklaySpacing.lg),
            child: Text(
              ErrorView.messageFor(context, _runner.error!),
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.danger,
              ),
            ),
          ),
        Padding(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          child: GoklayButton(
            label: strings.signOut,
            variant: GoklayButtonVariant.outlined,
            expand: true,
            onPressed: _runner.isBusy ? null : _signOut,
          ),
        ),
      ],
    );
  }

  Future<void> _setLanguage(String languageCode) async {
    final Dependencies dependencies = PartnerScope.of(context);
    final Locale locale = Locale(languageCode);
    dependencies.locale = locale;
    widget.onLanguageChanged(locale);
    await _runner.run(
      () => dependencies.account.updateProfile(language: languageCode),
    );
  }
}
