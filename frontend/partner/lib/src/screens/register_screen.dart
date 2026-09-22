import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../api/models/partner.dart';
import '../app_scope.dart';
import '../l10n/partner_strings.dart';

/// Signing up to carry orders.
///
/// There is no area to choose and nothing to approve against a map. **D1 from
/// the partner's side**: a rider may work anywhere in Bangladesh, and where
/// they can work is decided every time they report a location — so the form
/// asks for a name, a number and a vehicle, and nothing about geography.
class RegisterScreen extends StatefulWidget {
  /// Creates the form.
  const RegisterScreen({required this.onRegistered, super.key});

  /// Called with the partner the server created.
  final void Function(DeliveryPartner partner) onRegistered;

  @override
  State<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends State<RegisterScreen> {
  final ActionRunner _runner = ActionRunner();
  final TextEditingController _name = TextEditingController();
  final TextEditingController _phone = TextEditingController();
  final TextEditingController _vehicle = TextEditingController();

  @override
  void initState() {
    super.initState();
    _name.addListener(_onTyped);
    _phone.addListener(_onTyped);
  }

  @override
  void dispose() {
    _name.removeListener(_onTyped);
    _phone.removeListener(_onTyped);
    _name.dispose();
    _phone.dispose();
    _vehicle.dispose();
    _runner.dispose();
    super.dispose();
  }

  void _onTyped() => setState(() {});

  bool get _canRegister =>
      _name.text.trim().isNotEmpty && _phone.text.trim().isNotEmpty;

  Future<void> _register() async {
    DeliveryPartner? saved;
    final bool ok = await _runner.run(() async {
      saved = await PartnerScope.of(context).partner.register(
        name: _name.text.trim(),
        phone: _phone.text.trim(),
        vehicle: _vehicle.text.trim(),
      );
    });
    if (ok && saved != null && mounted) {
      widget.onRegistered(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final PartnerStrings strings = PartnerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.registerTitle,
      showBack: false,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            Text(strings.registerSubtitle, style: GoklayTextStyles.body),
            LabelledField(label: strings.riderName, controller: _name),
            LabelledField(
              label: strings.riderPhone,
              controller: _phone,
              keyboardType: TextInputType.phone,
            ),
            LabelledField(label: strings.vehicle, controller: _vehicle),
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
              label: strings.register,
              expand: true,
              onPressed: _canRegister && !_runner.isBusy ? _register : null,
            ),
          ],
        ),
      ),
    );
  }
}
