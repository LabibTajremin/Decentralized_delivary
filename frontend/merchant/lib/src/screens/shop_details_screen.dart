import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/merchant.dart';
import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/merchant_strings.dart';

/// Registering a shop, and editing its details afterwards — one form, because
/// they are the same fields and the same validation.
///
/// The shop types offered are the ones
/// `GET /v1/merchants/registration-requirements` lists, and the documents
/// each needs come from the same response. Neither is a constant here: which
/// papers a pharmacy must produce is a rule, and a rule in the app goes stale
/// the moment the regulator changes their mind (2.9).
///
/// The area, district and division are not fields. P02 resolves them from the
/// coordinate, and they are the D3 boundary the shop is forever served
/// within — not something a form should be able to state.
class ShopDetailsScreen extends StatefulWidget {
  /// Creates the form. A null [shop] registers a new one.
  const ShopDetailsScreen({required this.onSaved, this.shop, super.key});

  /// The shop being edited, or null when registering.
  final Merchant? shop;

  /// Called with the saved shop.
  final void Function(Merchant shop) onSaved;

  /// The label for a shop type, in the caller's language.
  ///
  /// The *set* of types is the server's; only their names are chrome.
  static String typeLabel(MerchantStrings strings, String type) =>
      switch (type) {
        'restaurant' => strings.typeRestaurant,
        'grocery' => strings.typeGrocery,
        'pharmacy' => strings.typePharmacy,
        _ => type,
      };

  @override
  State<ShopDetailsScreen> createState() => _ShopDetailsScreenState();
}

class _ShopDetailsScreenState extends State<ShopDetailsScreen> {
  final Store<RegistrationRequirements> _requirements =
      Store<RegistrationRequirements>();
  final ActionRunner _runner = ActionRunner();
  late final TextEditingController _name = TextEditingController(
    text: widget.shop?.name ?? '',
  );
  late final TextEditingController _phone = TextEditingController(
    text: widget.shop?.phone ?? '',
  );
  late final TextEditingController _email = TextEditingController(
    text: widget.shop?.email ?? '',
  );
  late final TextEditingController _line1 = TextEditingController(
    text: widget.shop?.line1 ?? '',
  );
  late final TextEditingController _line2 = TextEditingController(
    text: widget.shop?.line2 ?? '',
  );
  late final TextEditingController _lat = TextEditingController(
    text: widget.shop == null ? '' : '${widget.shop!.lat}',
  );
  late final TextEditingController _lng = TextEditingController(
    text: widget.shop == null ? '' : '${widget.shop!.lng}',
  );
  late String _type = widget.shop?.type ?? '';

  @override
  void initState() {
    super.initState();
    _lat.addListener(_onPointTyped);
    _lng.addListener(_onPointTyped);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _lat.removeListener(_onPointTyped);
    _lng.removeListener(_onPointTyped);
    for (final TextEditingController controller in <TextEditingController>[
      _name,
      _phone,
      _email,
      _line1,
      _line2,
      _lat,
      _lng,
    ]) {
      controller.dispose();
    }
    _requirements.dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onPointTyped() => setState(() {});

  Future<void> _load() async {
    final Dependencies dependencies = MerchantScope.of(context);
    await _requirements.load(() async {
      final ApiPage<RegistrationRequirements> page = await dependencies
          .merchant
          .requirements();
      return page.toAsyncData();
    });
  }

  /// The typed coordinate, or null when either half is not a number.
  (double, double)? get _point {
    final double? lat = double.tryParse(_lat.text.trim());
    final double? lng = double.tryParse(_lng.text.trim());
    return lat == null || lng == null ? null : (lat, lng);
  }

  bool get _canSave => _point != null && _type.isNotEmpty;

  Future<void> _save() async {
    final (double, double)? point = _point;
    if (point == null) {
      return;
    }
    final Dependencies dependencies = MerchantScope.of(context);
    Merchant? saved;
    final bool ok = await _runner.run(() async {
      saved = widget.shop == null
          ? await dependencies.merchant.register(
              name: _name.text.trim(),
              type: _type,
              phone: _phone.text.trim(),
              line1: _line1.text.trim(),
              lat: point.$1,
              lng: point.$2,
              email: _email.text.trim(),
              line2: _line2.text.trim(),
            )
          : await dependencies.merchant.updateDetails(
              name: _name.text.trim(),
              type: _type,
              phone: _phone.text.trim(),
              line1: _line1.text.trim(),
              lat: point.$1,
              lng: point.$2,
              email: _email.text.trim(),
              line2: _line2.text.trim(),
            );
    });
    if (ok && saved != null && mounted) {
      widget.onSaved(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    final bool isNew = widget.shop == null;
    return GoklayScaffold(
      title: isNew ? strings.registerTitle : strings.shopRow,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) =>
            AsyncView<RegistrationRequirements>(
              store: _requirements,
              onRetry: _load,
              builder:
                  (BuildContext context, RegistrationRequirements needed) =>
                      _form(strings, needed, isNew: isNew),
            ),
      ),
    );
  }

  Widget _form(
    MerchantStrings strings,
    RegistrationRequirements needed, {
    required bool isNew,
  }) {
    return ListView(
      padding: const EdgeInsets.all(GoklaySpacing.lg),
      children: <Widget>[
        if (isNew)
          Text(strings.registerSubtitle, style: GoklayTextStyles.body),
        LabelledField(label: strings.shopName, controller: _name),
        Text(
          strings.shopType,
          style: GoklayTextStyles.caption.copyWith(
            color: GoklayColors.textSecondary,
          ),
        ),
        for (final String type in needed.types)
          GoklayTapTarget(
            onTap: _runner.isBusy ? null : () => setState(() => _type = type),
            semanticLabel: ShopDetailsScreen.typeLabel(strings, type),
            child: Padding(
              padding: const EdgeInsets.symmetric(
                vertical: GoklaySpacing.xxs,
              ),
              child: Row(
                children: <Widget>[
                  Icon(
                    _type == type
                        ? Icons.check_circle
                        : Icons.circle_outlined,
                    size: 20,
                    color: _type == type
                        ? GoklayColors.brand
                        : GoklayColors.textSecondary,
                  ),
                  const SizedBox(width: GoklaySpacing.sm),
                  Expanded(
                    child: Text(
                      ShopDetailsScreen.typeLabel(strings, type),
                      style: GoklayTextStyles.body,
                    ),
                  ),
                  Text(
                    needed.byType[type]!.join(', '),
                    style: GoklayTextStyles.caption.copyWith(
                      color: GoklayColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ),
          ),
        LabelledField(
          label: strings.shopPhone,
          controller: _phone,
          keyboardType: TextInputType.phone,
        ),
        LabelledField(
          label: strings.shopEmail,
          controller: _email,
          keyboardType: TextInputType.emailAddress,
        ),
        LabelledField(label: strings.shopLine1, controller: _line1),
        LabelledField(label: strings.shopLine2, controller: _line2),
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
        if (_runner.error != null)
          Padding(
            padding: const EdgeInsets.only(top: GoklaySpacing.sm),
            child: Text(
              ErrorView.messageFor(context, _runner.error!),
              style: GoklayTextStyles.caption.copyWith(
                color: GoklayColors.danger,
              ),
            ),
          ),
        const SizedBox(height: GoklaySpacing.lg),
        GoklayButton(
          label: isNew ? strings.register : strings.save,
          expand: true,
          onPressed: _canSave && !_runner.isBusy ? _save : null,
        ),
      ],
    );
  }
}
