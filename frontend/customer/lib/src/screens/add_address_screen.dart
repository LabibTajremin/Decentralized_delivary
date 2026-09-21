import 'package:flutter/material.dart';
import 'package:goklay_core/goklay_core.dart';

import '../app_scope.dart';
import '../dependencies.dart';
import '../l10n/customer_strings.dart';

/// Saving a delivery address.
///
/// The area, district and division are **not** fields on this form. P02
/// resolves them from the coordinate, and a client that guessed would
/// eventually file an address under a division the order could not be served
/// from — D3 makes that boundary the hard edge of the whole system.
///
/// The point is confirmed before saving: `GET /v1/geo/resolve` answers 404 for
/// a coordinate outside every division, so the form can say so while the
/// customer is still looking at it rather than at checkout. The area name it
/// returns is shown as the server wrote it.
///
/// There is no map. This build has no maps plugin and the environment has no
/// key for one; picking a point on a map is platform work, and it would not
/// change what the app knows about the address.
class AddAddressScreen extends StatefulWidget {
  /// Creates the form.
  const AddAddressScreen({required this.onSaved, super.key});

  /// Called with the saved address.
  final void Function(Address address) onSaved;

  @override
  State<AddAddressScreen> createState() => _AddAddressScreenState();
}

class _AddAddressScreenState extends State<AddAddressScreen> {
  final TextEditingController _label = TextEditingController();
  final TextEditingController _recipient = TextEditingController();
  final TextEditingController _phone = TextEditingController();
  final TextEditingController _line1 = TextEditingController();
  final TextEditingController _line2 = TextEditingController();
  final TextEditingController _instructions = TextEditingController();
  final TextEditingController _lat = TextEditingController();
  final TextEditingController _lng = TextEditingController();
  final ActionRunner _runner = ActionRunner();
  Area? _area;

  @override
  void initState() {
    super.initState();
    // The save button is live only once there is a coordinate to send, so the
    // two fields that decide that have to rebuild it as they are typed into.
    _lat.addListener(_onPointTyped);
    _lng.addListener(_onPointTyped);
  }

  void _onPointTyped() => setState(() {});

  @override
  void dispose() {
    _lat.removeListener(_onPointTyped);
    _lng.removeListener(_onPointTyped);
    for (final TextEditingController controller in <TextEditingController>[
      _label,
      _recipient,
      _phone,
      _line1,
      _line2,
      _instructions,
      _lat,
      _lng,
    ]) {
      controller.dispose();
    }
    _runner.dispose();
    super.dispose();
  }

  /// The typed coordinate, or null when either half is not a number.
  (double, double)? get _point {
    final double? lat = double.tryParse(_lat.text.trim());
    final double? lng = double.tryParse(_lng.text.trim());
    return lat == null || lng == null ? null : (lat, lng);
  }

  Future<void> _resolve() async {
    final (double, double)? point = _point;
    if (point == null) {
      return;
    }
    final Dependencies dependencies = AppScope.of(context);
    await _runner.run(() async {
      final ApiPage<Area> page = await dependencies.discovery.resolve(
        lat: point.$1,
        lng: point.$2,
      );
      if (mounted) {
        setState(() => _area = page.value);
      }
    });
  }

  Future<void> _save() async {
    final (double, double)? point = _point;
    if (point == null) {
      return;
    }
    final Dependencies dependencies = AppScope.of(context);
    Address? saved;
    final bool ok = await _runner.run(() async {
      saved = await dependencies.account.addAddress(
        label: _label.text.trim(),
        recipientName: _recipient.text.trim(),
        recipientPhone: _phone.text.trim(),
        line1: _line1.text.trim(),
        line2: _line2.text.trim(),
        instructions: _instructions.text.trim(),
        lat: point.$1,
        lng: point.$2,
      );
    });
    if (ok && saved != null && mounted) {
      widget.onSaved(saved!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final CustomerStrings strings = CustomerLocalizations.of(context);
    return GoklayScaffold(
      title: strings.addAddress,
      body: ListenableBuilder(
        listenable: _runner,
        builder: (BuildContext context, _) => ListView(
          padding: const EdgeInsets.all(GoklaySpacing.lg),
          children: <Widget>[
            LabelledField(
              label: strings.addressLabelField,
              controller: _label,
            ),
            LabelledField(
              label: strings.recipientName,
              controller: _recipient,
            ),
            LabelledField(
              label: strings.recipientPhone,
              controller: _phone,
              keyboardType: TextInputType.phone,
            ),
            LabelledField(label: strings.addressLine1, controller: _line1),
            LabelledField(label: strings.addressLine2, controller: _line2),
            LabelledField(
              label: strings.addressInstructions,
              controller: _instructions,
              maxLines: 2,
            ),
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
            const SizedBox(height: GoklaySpacing.sm),
            GoklayButton(
              label: strings.confirm,
              variant: GoklayButtonVariant.outlined,
              expand: true,
              onPressed: _runner.isBusy ? null : _resolve,
            ),
            if (_area != null)
              Padding(
                padding: const EdgeInsets.only(top: GoklaySpacing.sm),
                child: Text(
                  '${_area!.areaName}, ${_area!.divisionName}',
                  style: GoklayTextStyles.caption.copyWith(
                    color: GoklayColors.brand,
                  ),
                ),
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
              label: strings.save,
              expand: true,
              onPressed: _runner.isBusy || _point == null ? null : _save,
            ),
          ],
        ),
      ),
    );
  }
}
