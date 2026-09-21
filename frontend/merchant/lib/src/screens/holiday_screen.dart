import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/merchant.dart';
import '../app_scope.dart';
import '../l10n/merchant_strings.dart';

/// Closing the shop for a holiday, and reopening it.
///
/// Being on holiday is what takes a shop off every customer's list, so it is
/// a call to the server rather than a local flag: an app that hid the shop by
/// itself would still be taking orders.
class HolidayScreen extends StatefulWidget {
  /// Creates the screen over [shop].
  const HolidayScreen({required this.shop, required this.onSaved, super.key});

  /// The shop.
  final Merchant shop;

  /// Called with the shop the server answered with.
  final void Function(Merchant shop) onSaved;

  @override
  State<HolidayScreen> createState() => _HolidayScreenState();
}

class _HolidayScreenState extends State<HolidayScreen> {
  final ActionRunner _runner = ActionRunner();
  late final TextEditingController _reason = TextEditingController(
    text: widget.shop.holiday?.reason ?? '',
  );
  late final TextEditingController _until = TextEditingController(
    text: widget.shop.holiday?.until?.toUtc().toIso8601String() ?? '',
  );

  @override
  void dispose() {
    _reason.dispose();
    _until.dispose();
    _runner.dispose();
    super.dispose();
  }

  Future<void> _apply({required bool clear}) async {
    Merchant? saved;
    final bool ok = await _runner.run(() async {
      saved = await MerchantScope.of(context).merchant.setHoliday(
        until: clear ? null : DateTime.tryParse(_until.text.trim()),
        reason: clear ? '' : _reason.text.trim(),
        clear: clear,
      );
    });
    if (ok && saved != null && mounted) {
      widget.onSaved(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final MerchantStrings strings = MerchantLocalizations.of(context);
    final bool onHoliday = widget.shop.holiday != null;
    return GoklayScaffold(
      title: strings.holidayTitle,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            if (onHoliday)
              Text(
                strings.onHoliday,
                style: GoklayTextStyles.body.copyWith(
                  color: GoklayColors.danger,
                ),
              ),
            LabelledField(label: strings.holidayReason, controller: _reason),
            LabelledField(label: 'until (ISO 8601)', controller: _until),
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
              label: strings.closeShop,
              expand: true,
              onPressed: _runner.isBusy ? null : () => _apply(clear: false),
            ),
            if (onHoliday) ...<Widget>[
              const SizedBox(height: GoklaySpacing.sm),
              GoklayButton(
                label: strings.reopenShop,
                variant: GoklayButtonVariant.outlined,
                expand: true,
                onPressed: _runner.isBusy ? null : () => _apply(clear: true),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
